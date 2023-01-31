package main

/*
#include <stdlib.h>

typedef char* (*message_callback) (char *msg);

typedef void (*success_callback) ();


static inline char* call_message_callback(message_callback ptr, char *msg) {
  return (ptr)(msg);
}

static inline void call_success_callback(success_callback ptr) {
  (ptr)();
}
*/
import "C"
import (
	"log"
	"runtime"
	"unsafe"

	gisquick "github.com/gisquick/gisquick-qgis-plugin/go"
	"github.com/gorilla/websocket"
)

var c *gisquick.Client

//export Start
func Start(url, user, password, clientInfo string, fn C.message_callback, success C.success_callback) int {
	c = gisquick.NewClient(url, user, password)
	c.ClientInfo = clientInfo
	c.OnMessageCallback = func(message []byte) string {
		cmsg := C.CString(string(message))
		defer C.free(unsafe.Pointer(cmsg))
		resp := C.call_message_callback(fn, cmsg)
		if resp == nil {
			return ""
		}
		return C.GoString(resp)
	}
	onConnectionEstabilished := func() {
		C.call_success_callback(success)
	}
	if err := c.Start(onConnectionEstabilished); err != nil {
		log.Println(err.Error())
		c = nil
		runtime.GC()
		return 1
	}
	runtime.GC()
	return 0
}

//export Stop
func Stop() {
	if c != nil {
		c.Stop()
		c = nil
	}
}

//export SendMessage
func SendMessage(msg string) {
	if c == nil {
		return
	}
	if err := c.SendRawMessage(websocket.TextMessage, []byte(msg)); err != nil {
		log.Printf("Failed to send WS message: %s\n", err)
		return
	}
}

func main() {}
