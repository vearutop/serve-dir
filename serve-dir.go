package main

import (
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	qrterminal "github.com/mdp/qrterminal/v3"
	"github.com/vearutop/httpzip"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		println(r.URL.Path)

		p := "." + r.URL.Path

		fi, err := os.Stat(p)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}

		if fi.IsDir() {
			if r.URL.Query().Get("zip") == "1" {
				if r.URL.Query().Get("recursive") == "1" {
					zipDir(w, wd, p, true)
				} else {
					zipDir(w, wd, p, false)
				}
			} else {
				listDir(w, wd, p)
			}
		} else {
			http.ServeFile(w, r, p)
		}
	})

	addr := "http://127.0.0.1:8099"

	m, err := Interfaces(false)
	if err != nil {
		log.Fatal(err)
	}

	for _, v := range m {
		addr = "http://" + v + ":8099"
	}

	fmt.Println(addr)
	qrterminal.Generate(addr, qrterminal.M, os.Stdout)

	if err := http.ListenAndServe(":8099", h); err != nil {
		log.Fatal(err)
	}
}

func listDir(w http.ResponseWriter, wd, p string) {
	dir := path.Join(wd, p)

	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!DOCTYPE html>\n<html><body>"))

	_, _ = w.Write([]byte(`<h1>Index of ` + dir + `</h1>`))

	l, err := os.ReadDir(p)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	hasFiles := false
	hasDirs := false
	for _, e := range l {
		if !e.IsDir() {
			hasFiles = true
		} else {
			hasDirs = true
		}

		if hasDirs && hasFiles {
			break
		}
	}

	if hasFiles {
		_, _ = w.Write([]byte(`<p><a href="?zip=1">Download files as uncompressed ZIP</a></p>`))
	}

	if hasDirs {
		_, _ = w.Write([]byte(`<p><a href="?zip=1&recursive=1">Download recursively as uncompressed ZIP</a></p>`))
	}

	_, _ = w.Write([]byte(`<table style="width:100%;"><tr><th align=left>Name</th><th align=left>Last Modified</th><th align=left>Size</th></tr>`))

	if p != "./" {
		_, _ = w.Write([]byte("<tr><td><a href=\"/" + html.EscapeString(path.Dir(p)) + "\">..</a></td><td>-</td><td>-</td></tr>\n"))
	}

	for _, e := range l {
		i, err := e.Info()
		if err != nil {
			_, _ = w.Write([]byte("<tr><td>" + err.Error() + "</td></tr>"))
			continue
		}

		_, _ = w.Write([]byte("<tr><td><a href=\"/" + html.EscapeString(path.Clean(p+"/"+url.PathEscape(e.Name()))) + "\">" + e.Name() + "</a></td><td>" + i.ModTime().Format(time.RFC3339) + "</td><td>" + strconv.Itoa(int(i.Size())) + "</td></tr>\n"))
	}

	_, _ = w.Write([]byte("</table></body></html>"))
}

func zipDir(rw http.ResponseWriter, wd, p string, recursive bool) {
	n := ""
	if p != "./" {
		n = path.Base(p)
	} else {
		n = path.Base(wd)
	}

	h := httpzip.NewHandler(n)
	h.OnError = func(err error) {
		log.Println(err.Error())
	}

	p = path.Clean(p)

	zipWalk(h, p, p, recursive)

	h.ServeHTTP(rw, nil)
}

func zipWalk(h *httpzip.Handler, basePath, strip string, recursive bool) {
	if strip == "." {
		strip = "./"
	}

	l, err := os.ReadDir(basePath)
	if err != nil {
		log.Println(err.Error())
		return
	}

	for _, e := range l {
		fn := path.Join(basePath, e.Name())
		i, err := os.Stat(fn)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		if i.IsDir() {
			if recursive {
				zipWalk(h, fn, strip, true)
			}

			continue
		}

		err = h.AddFile(httpzip.FileSource{
			Path:     strings.TrimPrefix(strings.TrimPrefix(fn, strip), "/"),
			Modified: i.ModTime(),
			Size:     i.Size(),
			Data: func(w io.Writer) error {
				src, err := os.Open(fn)
				if err != nil {
					log.Println(err.Error())
					return err
				}
				defer func() {
					if err := src.Close(); err != nil {
						log.Println(err.Error())
					}
				}()

				if _, err = io.Copy(w, src); err != nil {
					return err
				}

				return nil
			},
		})
		if err != nil {
			log.Println(err.Error())
		}
	}
}

// Interfaces returns a `name:ip` map of the suitable interfaces found
func Interfaces(listAll bool) (map[string]string, error) {
	names := make(map[string]string)
	ifaces, err := net.Interfaces()
	if err != nil {
		return names, err
	}
	re := regexp.MustCompile(`^(veth|br\-|docker|lo|EHC|XHC|bridge|gif|stf|p2p|awdl|utun|tun|tap)`)
	for _, iface := range ifaces {
		if !listAll && re.MatchString(iface.Name) {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		ip, err := FindIP(iface)
		if err != nil {
			continue
		}
		names[iface.Name] = ip
	}
	return names, nil
}

// FindIP returns the IP address of the passed interface, and an error
func FindIP(iface net.Interface) (string, error) {
	var ip string
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.IsLinkLocalUnicast() {
				continue
			}
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				continue
			}
			// Use IPv6 only if an IPv4 hasn't been found yet.
			// This is eventually overwritten with an IPv4, if found (see above)
			if ip == "" {
				ip = "[" + ipnet.IP.String() + "]"
			}
		}
	}
	if ip == "" {
		return "", errors.New("unable to find an IP for this interface")
	}
	return ip, nil
}
