package servers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

type KapingerHTTPServer struct {
	port int
}

func NewKapingerHTTPServer(port int) *KapingerHTTPServer {
	return &KapingerHTTPServer{
		port: port,
	}
}

func (k *KapingerHTTPServer) Start(ctx context.Context) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(getResponse(r.RemoteAddr, "http"))
		if err != nil {
			fmt.Println(err)
		}
	})

	addr := ":" + strconv.Itoa(k.port)

	log.Printf("[HTTP] Listening on %+v\n", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(handler),
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()

	<-ctx.Done()
	err := server.Shutdown(ctx)
	if err != nil {
		return err
	}

	return nil
}
