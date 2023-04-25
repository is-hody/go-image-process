package server

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"github.com/go-kratos/kratos/v2/errors"
	"go-image-process/internal/conf"
	"go-image-process/internal/service"
	"net/http"

	"github.com/go-kratos/kratos/v2/middleware/recovery"
	transportHttp "github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new a HTTP server.
func NewHTTPServer(
	c *conf.Bootstrap,
	image service.ImageInterface,
) *transportHttp.Server {
	var opts = []transportHttp.ServerOption{
		transportHttp.Middleware(
			recovery.Recovery(),
		),
		transportHttp.ErrorEncoder(CustomErrorEncoder),
		transportHttp.RequestDecoder(DefaultRequestDecoder),
		transportHttp.ResponseEncoder(DefaultResponseEncoder),
	}
	if c.GetServer().Http.Network != "" {
		opts = append(opts, transportHttp.Network(c.GetServer().Http.Network))
	}
	if c.GetServer().Http.Addr != "" {
		opts = append(opts, transportHttp.Address(c.GetServer().Http.Addr))
	}
	if c.GetServer().Http.Timeout != nil {
		opts = append(opts, transportHttp.Timeout(c.GetServer().Http.Timeout.AsDuration()))
	}
	srv := transportHttp.NewServer(opts...)
	router := srv.Route("/")
	router.GET("/health", func(context transportHttp.Context) error {
		return context.String(http.StatusOK, "success")
	})
	router.POST("/image", handler(image.ImageHandler))
	return srv
}

func DefaultRequestDecoder(r *http.Request, v interface{}) error {
	var dec = decoder.NewStreamDecoder(r.Body)
	if err := dec.Decode(&v); err != nil {
		return errors.BadRequest("CODEC", fmt.Sprintf("body unmarshal %s", err.Error()))
	}
	return nil
}

type ErrReply struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

func CustomErrorEncoder(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	res := &ErrReply{}
	se := errors.FromError(err)
	res = &ErrReply{Code: int(se.Code), Message: se.Message, Reason: se.Reason}
	w.WriteHeader(http.StatusOK)
	output, err := sonic.Marshal(&res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	_, _ = w.Write(output)
}

func DefaultResponseEncoder(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if v == nil {
		return nil
	}
	if rd, ok := v.(transportHttp.Redirector); ok {
		url, code := rd.Redirect()
		http.Redirect(w, r, url, code)
		return nil
	}
	output, err := sonic.Marshal(&v)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(output)
	if err != nil {
		return err
	}
	return nil
}

func handler(f func(ctx context.Context, httpContext transportHttp.Context) (interface{}, error)) transportHttp.HandlerFunc {
	return func(httpContext transportHttp.Context) error {
		h := httpContext.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return f(ctx, httpContext)
		})
		_, err := h(httpContext, nil)
		if err != nil {
			return err
		}
		return nil
	}
}
