package utils

import (
	storage "ccache-backend-client/storage"
	"net"
)

func TmpDetermineBackend(url string) storage.HttpStorageBackend {
	var tmp storage.HttpStorageBackend
	tmp.Create(url, nil)
	return tmp
}

type Handler interface {
	Handle(storage.Message)
}

type SocketHandler struct {
	node net.Conn
}

func (h *SocketHandler) Handle(msg storage.Message) {

}

type BackendHandler struct {
	node storage.Backend
}

func (h *BackendHandler) Handle(msg storage.Message) {
	msg.Write(h.node)
}
