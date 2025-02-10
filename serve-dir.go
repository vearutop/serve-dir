package main

import (
	"errors"
	"flag"
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
	"sync/atomic"
	"time"

	qrterminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/bool64/progress"
	"github.com/vearutop/httpzip"
)

func main() {
	var dlzip string

	flag.StringVar(&dlzip, "dlzip", "", "URL to ZIP file. Archive is extracted into current directory.")
	flag.Parse()

	if dlzip != "" {
		if err := dlZip(dlzip); err != nil {
			log.Fatal(err)
		}
		return
	}

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

	fmt.Println(addr, "\n")

	qrterminal.New().Get(addr).Print()

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

func dlZip(u string) error {
	resp, err := http.Get(u)
	if err != nil {
		return err
	}

	total := resp.ContentLength
	filesDone := int64(0)
	maxLine := int64(0)

	pr := progress.Progress{}

	defer resp.Body.Close()

	cr := progress.NewCountingReader(resp.Body)
	cr.SetLines(nil)

	pr.Print = func(s progress.Status) {
		if s.Task != "" {
			s.Task += ": "
		}

		res := fmt.Sprintf(s.Task+"%.1f%% bytes read, %d files processed, %.1f files/s, %.1f MB/s, elapsed %s, remaining %s",
			s.DonePercent, s.LinesCompleted, s.SpeedLPS, s.SpeedMBPS,
			s.Elapsed.Round(10*time.Millisecond).String(), s.Remaining.String())

		fmt.Print("\r" + strings.Repeat(" ", int(atomic.LoadInt64(&maxLine))) + "\r")
		fmt.Print(res)

		if atomic.LoadInt64(&maxLine) < int64(len(res)) {
			atomic.StoreInt64(&maxLine, int64(len(res)))
		}
	}

	pr.Start(func(t *progress.Task) {
		t.TotalBytes = func() int64 {
			return total
		}

		t.CurrentBytes = cr.Bytes
		t.CurrentLines = func() int64 {
			return atomic.LoadInt64(&filesDone)
		}
	})

	zr := httpzip.NewStreamReader(cr)

	for {
		e, err := zr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("get next entry: %w", err)
		}

		if !e.IsDir() {
			fmt.Print("\r" + strings.Repeat(" ", int(atomic.LoadInt64(&maxLine))) + "\r")
			log.Println("downloading:", e.Name)

			rc, err := e.Open()
			if err != nil {
				return fmt.Errorf("open zip entry: %w", err)
			}

			if err := os.MkdirAll(path.Dir(e.Name), 0o755); err != nil {
				return fmt.Errorf("mkdirall: %w", err)
			}

			f, err := os.Create(e.Name)
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}

			w, err := io.Copy(f, rc)
			if err != nil {
				return fmt.Errorf("stream zip file (%d): %w )", w, err)
			}

			fmt.Print("\r" + strings.Repeat(" ", int(atomic.LoadInt64(&maxLine))) + "\r")
			log.Println("file length:", w)
			atomic.AddInt64(&filesDone, 1)

			if err := f.Close(); err != nil {
				return fmt.Errorf("close file: %w", err)
			}

			if err := rc.Close(); err != nil {
				fmt.Errorf("close zip entry reader: %w", err)
			}
		}
	}

	pr.Stop()

	return nil
}
