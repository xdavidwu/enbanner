package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"golang.org/x/net/html"
)

var (
	upstream = flag.String("u", "http://127.0.0.1:8000", "Upstream URL")
	addr     = flag.String("l", "0.0.0.0:8001", "Listen on address")
	msg      = flag.String("m", "Production", "Banner message")
	color    = flag.String("c", "red", "CSS color of banner")

	banner = []byte{}
)

const (
	bannerFmt = `<template class="enbanner-template">
	<style>
	div {
		background-color: %s;
		color: white;
		position: absolute;
		top: calc(-20px / sqrt(2));
		left: calc(-1 * (120px - 120px / sqrt(2)) - 20px / sqrt(2));
		z-index: 32767;
		transform: rotate(-45deg);
		width: 120px;
		text-align: center;
		line-height: 20px;
		font-size: 12px;
		transform-origin: top right;
	}
	b {
		opacity: 0.8;
	}
	</style>
	<div>%s <b>🞪</b></div>
</template>
<span class="enbanner-host"></span>
<script>
	const host = document.querySelector('.enbanner-host');
	const shadow = host.attachShadow({ mode: 'open' });
	const template = document.querySelector('.enbanner-template');
	shadow.appendChild(template.content);
	shadow.querySelector('b').addEventListener('click', () => host.remove());
</script>`
)

func modifyResponse(r *http.Response) error {
	c := r.Header.Get("Content-Type")
	if c == "" { // maybe a redirect?
		return nil
	}
	m, _, err := mime.ParseMediaType(c)
	if err != nil {
		return err
	}
	if m != "text/html" {
		return nil
	}
	// TODO handle compression?

	b := r.Body
	t := html.NewTokenizer(b)
	defer b.Close()

	o := bytes.Buffer{}
	for {
		ttype := t.Next()
		if ttype == html.ErrorToken {
			if err = t.Err(); err != io.EOF {
				return err
			}
			break
		}

		if ttype == html.StartTagToken {
			tag, _ := t.TagName()
			if bytes.Equal(tag, []byte("body")) {
				o.Write(t.Raw())
				o.Write(banner)
				o.Write(t.Buffered())
				if _, err = io.Copy(&o, b); err != nil {
					return err
				}
				break
			}
		}
		o.Write(t.Raw())
	}
	r.Body = io.NopCloser(&o)
	return nil
}

func main() {
	flag.Parse()
	u, err := url.Parse(*upstream)
	if err != nil {
		panic(err)
	}

	banner = fmt.Appendf(banner, bannerFmt, *color, html.EscapeString(*msg))

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}

	p := httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			r.Out.Host = r.In.Host
			r.Out.Header.Del("Accept-Encoding")
			r.SetXForwarded()
		},
		ModifyResponse: modifyResponse,
	}

	s := http.Server{Handler: &p}
	if err := s.Serve(l); err != nil {
		panic(err)
	}
}
