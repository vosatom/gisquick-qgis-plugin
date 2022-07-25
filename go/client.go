package gisquick

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gorilla/websocket"
)

// Gisquick plugin client
type Client struct {
	Server            string
	User              string
	Password          string
	ClientInfo        string
	httpClient        *http.Client
	WsConn            *websocket.Conn
	interrupt         chan int
	OnMessageCallback func([]byte) string
	messageHandlers   map[string]messageHandler
	cancelUpload      context.CancelFunc
}

type messageHandler func(msg message) error

type message struct {
	Type   string          `json:"type"`
	Status int             `json:"status,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type genericMessage struct {
	Type   string      `json:"type"`
	Status int         `json:"status,omitempty"`
	Data   interface{} `json:"data"`
}

// Creates a new Gisquick plugin client
func NewClient(url, user, password string) *Client {
	c := Client{}
	c.Server = url
	c.User = user
	c.Password = password
	cookieJar, _ := cookiejar.New(nil)
	c.httpClient = &http.Client{Jar: cookieJar}
	c.registerHandlers()
	return &c
}

// sends message with status code 200 ("ok")
func (c *Client) sendResponseMessage(msgType string, data interface{}) error {
	return c.WsConn.WriteJSON(genericMessage{Type: msgType, Status: 200, Data: data})
}

// sends error message
func (c *Client) sendErrorMessage(msgType string, data string) error {
	return c.WsConn.WriteJSON(genericMessage{Type: msgType, Status: 500, Data: data})
}

// send message to plugin handler and return response message
func (c *Client) propagateMessage(msgType string, data interface{}) (*message, error) {
	request, err := json.Marshal(genericMessage{Type: msgType, Data: data})
	if err != nil {
		return nil, err
	}
	resp := c.OnMessageCallback(request)
	if resp == "" {
		return nil, errors.New("Empty response")
	}
	var msg message
	if err = json.Unmarshal([]byte(resp), &msg); err != nil {
		return nil, fmt.Errorf("Invalid message: %s (%s)", resp, err)
	}
	return &msg, nil
}

func (c *Client) readTextData(data string) (string, error) {
	var msg message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		return "", err
	}
	var value string
	if err := json.Unmarshal(msg.Data, &value); err != nil {
		return "", err
	}
	return value, nil
}

/* Message handlers */

func (c *Client) registerHandlers() {
	c.messageHandlers = make(map[string]messageHandler)
	c.messageHandlers["PluginStatus"] = c.handlePluginStatus
	c.messageHandlers["ProjectFiles"] = c.handleProjectFiles
	c.messageHandlers["AbortUpload"] = c.handleAbortUpload
	c.messageHandlers["UploadFiles"] = c.handleUploadFiles
}

func (c *Client) handlePluginStatus(msg message) error {
	data := map[string]string{"client": c.ClientInfo}
	return c.sendResponseMessage("PluginStatus", data)
}

func (c *Client) handleProjectFiles(msg message) error {
	type filesMsg struct {
		Directory string     `json:"directory"`
		Files     []FileInfo `json:"files"`
	}

	projDirMsg, err := c.propagateMessage("ProjectDirectory", nil)
	if err != nil {
		return errors.New("Failed to get project directory")
	}
	if projDirMsg.Status != 200 {
		projDirMsg.Type = msg.Type
		return c.WsConn.WriteJSON(projDirMsg)
	}
	var directory string
	json.Unmarshal(projDirMsg.Data, &directory)
	files, err := ListDir(directory, true)

	if err != nil {
		return err
	}
	for i, f := range *files {
		(*files)[i].Path = filepath.ToSlash(f.Path)
	}
	data := filesMsg{Directory: directory, Files: *files}
	return c.sendResponseMessage(msg.Type, data)
}

func (c *Client) handleAbortUpload(msg message) error {
	if c.cancelUpload != nil {
		c.cancelUpload()
		c.cancelUpload = nil
	}
	return nil
}

func (c *Client) handleUploadFiles(msg message) error {
	type Params struct {
		Project string     `json:"project"`
		Files   []FileInfo `json:"files"`
	}
	var params Params
	if err := json.Unmarshal(msg.Data, &params); err != nil {
		return err
	}

	projDirMsg, err := c.propagateMessage("ProjectDirectory", nil)
	if err != nil {
		return errors.New("Failed to get project directory")
	}
	if projDirMsg.Status != 200 {
		projDirMsg.Type = msg.Type
		return c.WsConn.WriteJSON(projDirMsg)
	}
	var directory string
	json.Unmarshal(projDirMsg.Data, &directory)

	go func() {
		readBody, writeBody := io.Pipe()
		defer readBody.Close()

		writer := multipart.NewWriter(writeBody)
		errChan := make(chan error, 1)

		go func() {
			compressRegex := regexp.MustCompile("(?i).*\\.(qgs|xml|csv|svg|tif|dbf|json|sqlite|gpkg|geojson)$")
			defer writeBody.Close()

			changesUpdated := false
			for i, f := range params.Files {
				if f.Mtime == 0 {
					p := filepath.Join(directory, f.Path)
					finfo, err := os.Stat(p)
					if err != nil {
						errChan <- err
						return
					}
					params.Files[i].Mtime = finfo.ModTime().Unix()
					params.Files[i].Size = finfo.Size()
					if f.Hash == "" {
						hash, err := Checksum(p)
						if err != nil {
							errChan <- err
							return
						}
						params.Files[i].Hash = hash
					}
					changesUpdated = true
				}
			}
			if changesUpdated {
				data, err := json.Marshal(params)
				if err != nil {
					errChan <- err
					return
				}
				writer.WriteField("changes", string(data))
			} else {
				writer.WriteField("changes", string(msg.Data))
			}

			for _, f := range params.Files {
				// ext := filepath.Ext(f.Path)
				fileOsPath := filepath.FromSlash(f.Path)
				useCompression := compressRegex.Match([]byte(f.Path))
				if useCompression {
					mh := make(textproto.MIMEHeader)
					mh.Set("Content-Type", "application/octet-stream")
					mh.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s.gz"`, f.Path, f.Path))
					part, _ := writer.CreatePart(mh)
					gzpart := gzip.NewWriter(part)
					err := CopyFile(gzpart, filepath.Join(directory, fileOsPath))
					gzpart.Close()
					if err != nil {
						errChan <- err
						return
					}
				} else {
					part, err := writer.CreateFormFile(f.Path, f.Path)
					if err != nil {
						errChan <- err
						return
					}
					if err = CopyFile(part, filepath.Join(directory, fileOsPath)); err != nil {
						errChan <- err
						return
					}
				}
			}
			errChan <- writer.Close()
		}()

		url := fmt.Sprintf("%s/api/project/upload/%s", c.Server, params.Project)
		req, _ := http.NewRequest("POST", url, readBody)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		ctx, cancel := context.WithCancel(context.Background())
		req = req.WithContext(ctx)
		c.cancelUpload = cancel

		resp, err := c.httpClient.Do(req)
		if err != nil {
			log.Printf("Failed to execute upload request: %s\n", err)
			c.sendErrorMessage("UploadError", "Upload error")
			return
		}
		defer resp.Body.Close()
		c.cancelUpload = nil

		log.Println("Upload response:", resp.StatusCode)

		respData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read upload response: %s\n", err)
		}
		if resp.StatusCode >= 400 {
			if err = c.sendErrorMessage("UploadError", string(respData)); err != nil {
				log.Printf("Failed to send error message: %s\n", err)
			}
		}
		err = <-errChan
		if err != nil {
			log.Println(err)
		}
	}()
	return nil
}

/* Normal methods */

func (c *Client) login() error {
	form := url.Values{"username": {c.User}, "password": {c.Password}}
	url := fmt.Sprintf("%s/api/auth/login/", c.Server)
	resp, err := c.httpClient.PostForm(url, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("Authentication failed")
	}
	return nil
}

func (c *Client) logout() error {
	url := fmt.Sprintf("%s/api/auth/logout/", c.Server)
	_, err := c.httpClient.Get(url)
	if err != nil {
		return err
	}
	return nil
}

// Starts a websocket connection with server and handles incomming messages
func (c *Client) Start(OnConnectionEstabilished func()) error {
	err := c.login()
	if err != nil {
		return err
	}
	defer c.logout()

	c.interrupt = make(chan int, 1)

	u, _ := url.Parse(c.Server)
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = fmt.Sprintf("/ws/plugin")

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 30 * time.Second,
		Jar:              c.httpClient.Jar,
	}
	header := make(http.Header, 1)
	header.Set("User-Agent", c.ClientInfo)
	wsConn, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		return err
	}
	if OnConnectionEstabilished != nil {
		OnConnectionEstabilished()
	}

	c.WsConn = wsConn
	defer wsConn.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)

		// c.OnMessageCallback([]byte("{ \"type\": \"connection:success\"}"))

		for {
			_, rawMessage, err := wsConn.ReadMessage()
			if err != nil {
				log.Println("WS read error:", err)
				return
			}
			var msg message
			if err = json.Unmarshal(rawMessage, &msg); err != nil {
				log.Printf("Invalid message: %s\n", rawMessage)
				continue
			}
			// log.Println("Msg type: ", msg.Type)
			// log.Printf("Received: %s\n", message)
			msgHandler, ok := c.messageHandlers[msg.Type]
			if ok {
				if err := msgHandler(msg); err != nil {
					log.Println(err)
					c.sendErrorMessage(msg.Type, err.Error())
				}
				continue
			}
			// possible issue if executed in different thread?
			resp := c.OnMessageCallback(rawMessage)
			if resp != "" {
				c.WsConn.WriteMessage(websocket.TextMessage, []byte(resp))
			}
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return nil
		case <-c.interrupt:
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("WS close error:", err)
				return nil
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return nil
		}
	}
}

// Closes websocket connection
func (c *Client) Stop() {
	c.interrupt <- 1
}
