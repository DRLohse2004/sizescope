//go:build dev

// Development server: serves the frontend and exposes App methods over HTTP
// so the UI can be tested in a normal browser. Run: go run -tags dev . [port]
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
)

func main() {
	app := NewApp()
	port := "34115"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	app.startArg = ""
	v := reflect.ValueOf(app)
	http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		m := v.MethodByName(strings.TrimPrefix(r.URL.Path, "/api/"))
		if !m.IsValid() {
			http.Error(w, "unknown method", 404)
			return
		}
		var raw []json.RawMessage
		json.NewDecoder(r.Body).Decode(&raw)
		in := make([]reflect.Value, m.Type().NumIn())
		for i := range in {
			p := reflect.New(m.Type().In(i))
			if i < len(raw) {
				json.Unmarshal(raw[i], p.Interface())
			}
			in[i] = p.Elem()
		}
		var out []reflect.Value
		failed := false
		func() {
			defer func() {
				if e := recover(); e != nil {
					println("PANIC", r.URL.Path, fmt.Sprint(e))
					failed = true
					http.Error(w, "panic", 500)
				}
			}()
			out = m.Call(in)
		}()
		if failed {
			return
		}
		res := map[string]any{}
		for _, o := range out {
			if err, ok := o.Interface().(error); ok || (o.Type().String() == "error") {
				if err != nil {
					res["error"] = err.Error()
				}
				continue
			}
			res["result"] = o.Interface()
		}
		if err := json.NewEncoder(w).Encode(res); err != nil {
			println("ENCODE", r.URL.Path, err.Error())
		}
	})
	http.Handle("/dev/", http.StripPrefix("/dev/", http.FileServer(http.Dir("dev"))))
	http.Handle("/", http.FileServer(http.Dir("frontend/dist")))
	println("http://localhost:" + port + "/dev/index.html")
	http.ListenAndServe("127.0.0.1:"+port, nil)
}
