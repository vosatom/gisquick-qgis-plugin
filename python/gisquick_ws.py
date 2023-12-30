import os
import sys
import json
import ctypes
import _ctypes
import platform
from traceback import TracebackException, format_exception, format_exception_only
from qgis.core import NULL

system = platform.system()
if system == "Linux":
    lib_ext = "so"
elif system == "Windows":
    lib_ext = "dll"
elif system == "Darwin":
    lib_ext = "dylib"
else:
    raise RuntimeError("Not supported OS!")

lib_path = os.path.join(os.path.dirname(__file__), "gisquick.%s" % lib_ext)

class GoString(ctypes.Structure):
    _fields_ = [("p", ctypes.c_char_p), ("n", ctypes.c_longlong)]

def go_string(s):
    return GoString(s.encode("utf-8"), len(s))

# Custom JSON encoder that can handle QGIS NULL values
class GisquickJSONEncoder(json.JSONEncoder):
    def default(self, obj):
        if obj == NULL:
            return None
        return super().default(obj)


class WsError(Exception):
    def __init__(self, msg, code=500):
        super().__init__(msg)
        self.code = code


class GisquickWs():
    """Wrapper for Gisquick websocket compiled lib"""

    _lib = None

    def _load_lib(self):
        if not self._lib:
            # ctypes._reset_cache()
            # self._lib = ctypes.CDLL(lib_path)
            self._lib = ctypes.cdll.LoadLibrary(lib_path)

    def _unload_lib(self):
        return
        # Unloading doesn't seem to work properly
        # https://github.com/golang/go/issues/11100
        """
        if self._lib:
            if lib_path.endswith(".so"):
                # print("handle", self._lib._handle)
                _ctypes.dlclose(self._lib._handle)
            elif lib_path.endswith(".dll"):
                from ctypes import wintypes
                ctypes.windll.kernel32.FreeLibrary.argtypes = [wintypes.HMODULE]
                ctypes.windll.kernel32.FreeLibrary(self._lib._handle)
                # _ctypes.FreeLibrary(self._lib._handle)
            del self._lib
        """

    def start(self, url, username, password, client_info, callback, success_callback):
        self._load_lib()

        @ctypes.CFUNCTYPE(ctypes.c_char_p, ctypes.c_char_p)
        def callback_wrapper(msg):
            msg = json.loads(msg)
            resp = {
                "type": msg["type"]
            }
            if "id" in msg:
                resp["id"] = msg["id"]
            try:
                ret_value = callback(msg) or ""
                resp["status"] = 200
                resp["data"] = ret_value
            except Exception as e:
                # t = TracebackException.from_exception(e)
                t = TracebackException.from_exception(e.__cause__ if e.__cause__ else e)
                resp["status"] = e.code if isinstance(e, WsError) else 500
                resp["data"] = str(e)
                resp["traceback"] = ''.join(t.format())
                exc_type, exc_value, exc_traceback = sys.exc_info()
                # tb = format_exception(
                #     exc_type,
                #     exc_value if isinstance(exc_value, exc_type) else exc_type(exc_value),
                #     exc_traceback
                # )
                # print(''.join(tb))
                # print([l for l in t.format()])

                # if e.__cause__:
                #     t = TracebackException.from_exception(e.__cause__)
                #     resp["traceback"] = "".join(t.format())
            return json.dumps(resp, cls=GisquickJSONEncoder).encode("utf-8")

        @ctypes.CFUNCTYPE(ctypes.c_void_p)
        def success_callback_wrapper():
            success_callback()

        try:
            return self._lib.Start(
                go_string(url),
                go_string(username),
                go_string(password),
                go_string(client_info),
                callback_wrapper,
                success_callback_wrapper
            )
        finally:
            self._unload_lib()

    def stop(self):
        if self._lib:
            self._lib.Stop()

    def send(self, name, data=None):
        msg = {
            "type": name
        }
        if data is not None:
            msg["data"] = data
        if self._lib:
            self._lib.SendMessage(go_string(json.dumps(msg)))

gisquick_ws = GisquickWs()


"""
Alternative version with separate process. Doesn't work properly on Windows,
somehow it starts a new qgis app... However for development on Linux it could be
more convinient, cause you do not need to restart qgis when rebuilding native lib.
"""
"""
import multiprocessing 

def go(url, username, password, parent_conn, child_conn):
    lib = ctypes.cdll.LoadLibrary(lib_path)

    @ctypes.CFUNCTYPE(ctypes.c_char_p, ctypes.c_char_p)
    def callback_wrapper(msg):
        try:
            # print("received MSG", msg.decode())
            child_conn.send(msg.decode())
            reply = child_conn.recv()
            return reply.encode("utf-8")
        except Exception as e:
            traceback.print_exc()
            # traceback.print_stack()
            # raise
    return lib.Start(go_string(url), go_string(username), go_string(password), callback_wrapper)


class GisquickWs2():
    p1 = None
    def start(self, url, username, password, callback):
        print("Start")
        parent_conn, child_conn = multiprocessing.Pipe()
        p1 = multiprocessing.Process(target=go, args=(url, username, password, parent_conn, child_conn))
        p1.start()
        self.p1 = p1
        self.parent_conn = parent_conn

        while True:
            msg = parent_conn.recv()
            parts = msg.split(":", 1)
            msg_type = parts[0]
            payload = parts[1] if len(parts) == 2 else None
            print("Received:", msg_type)
            resp = callback(msg_type, payload)
            if resp:
                self.send(resp)

        return p1.join()

    def send(self, name, data=None):
        msg = "%s:%s" % (name, data) if data else name
        self.parent_conn.send(msg)

    def stop(self):
        self.p1.terminate()


gisquick_ws = GisquickWs2()
"""
