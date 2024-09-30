package main

import (
	"archive/zip"
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

	qrterminal "github.com/mdp/qrterminal/v3"
)

func main() {
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
				zipDir(w, p)
			} else {
				listDir(w, p)
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

func listDir(w http.ResponseWriter, p string) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("<!DOCTYPE html>\n<html><body>"))

	l, err := os.ReadDir(p)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	for _, e := range l {
		w.Write([]byte("<a href=\"/" + html.EscapeString(path.Clean(p+"/"+url.PathEscape(e.Name()))) + "\">" + e.Name() + "</a><br>\n"))
	}

	w.Write([]byte(`<p><a href="?zip=1">Download all as uncompressed ZIP</a></p>`))

	w.Write([]byte("</body></html>"))
}

func zipDir(rw http.ResponseWriter, p string) {
	l, err := os.ReadDir(p)
	if err != nil {
		rw.Write([]byte(err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/zip")
	rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"files.zip\""))

	// Create a new zip archive.
	w := zip.NewWriter(rw)
	defer func() {
		// Make sure to check the error on Close.
		clErr := w.Close()
		if clErr != nil {
			println(clErr.Error())
		}
	}()

	copyImg := func(f io.Writer, fn string) error {
		src, err := os.Open(fn)
		if err != nil {
			println(err.Error())
			return err
		}
		defer func() {
			if err := src.Close(); err != nil {
				println(err.Error())
			}
		}()

		if _, err = io.Copy(f, src); err != nil {
			return err
		}

		return nil
	}

	for _, e := range l {
		f, err := w.CreateHeader(&zip.FileHeader{
			Name:   e.Name(),
			Method: zip.Store,
		})
		if err != nil {
			println(err.Error())
			return
		}

		if err := copyImg(f, path.Join(p, e.Name())); err != nil {
			println(err.Error())
			return
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
