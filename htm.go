// Copyright (c) 2025 Christopher Milan.
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
// WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY
// SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
// WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION
// OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN
// CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

package main

import (
  "bufio"
  "flag"
  "fmt"
  "log"
  "net/http"
  "net/http/httputil"
  "net/url"
  "os"
  "strings"
)

func usage() {
  fmt.Fprintf(os.Stderr, "usage: htm [options]\n")
  flag.PrintDefaults()
  os.Exit(1)
}

var (
  port       = flag.Int("port", 8080, "address to serve on")
  configPath = flag.String("config", "/etc/htm/htm.conf", "configuration file")
)

type HTMProxy map[string]url.URL

func newHTMProxy(filename string) (HTMProxy, error) {
  result := make(map[string]url.URL)

  file, err := os.Open(filename)
  if err != nil {
    return nil, err
  }
  defer file.Close()

  scanner := bufio.NewScanner(file)
  lineNum := 0
  for scanner.Scan() {
    lineNum++
    line := scanner.Text()
    line = strings.TrimSpace(line)

    // ignore empty lines and comments
    if line == "" || strings.HasPrefix(line, "#") {
      continue
    }

    fields := strings.Fields(line)
    if len(fields) < 2 {
      log.Printf("WARN: Ignoring invalid line %s:%d (insufficient fields)", filename, lineNum)
      continue
    }

    target, err := url.Parse(fields[0])
    if err != nil || target.Scheme == "" || target.Host == "" {
      log.Printf("WARN: Ignoring invalid line %s:%d (invalid url)", filename, lineNum)
      continue
    }

    for _, host := range fields[1:] {
      if strings.HasPrefix(host, "#") {
        break
      }
      if _, exists := result[host]; exists {
        log.Printf("WARN: Hostname '%s' was assigned multiple ports, using %d", host, port)
      }
      result[host] = *target
    }
  }

  if err := scanner.Err(); err != nil {
    return nil, err
  }

  return result, nil
}

func (config HTMProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  hostname := r.Host

  var proxyUrl url.URL
  found := false

  for host, url := range config {
    if strings.HasSuffix(hostname, host) {
      found = true
      proxyUrl = url
      break
    }
  }

  if !found {
    http.Error(w, "Bad Gateway", http.StatusBadGateway)
    log.Printf("WARN: failed to proxy request: %s", hostname)
    return
  }

  log.Printf("INFO: proxying %s to %s", hostname, proxyUrl.String())
  proxy := httputil.NewSingleHostReverseProxy(&proxyUrl)
  proxy.ServeHTTP(w, r)
}

func main() {
  flag.Usage = usage
  flag.Parse()

  proxy, err := newHTMProxy(*configPath)
  if err != nil {
    log.Println("ERR: Could not read config:", err)
    return
  }

  log.Printf("INFO: htm server started on :%d", *port)

  log.Fatal(fmt.Sprint("ERR: ", http.ListenAndServe(fmt.Sprint(":", *port), proxy)))
}
