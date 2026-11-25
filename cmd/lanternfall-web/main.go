//go:build js && wasm

// Command lanternfall-web is the in-browser world: the Go game engine
// compiled to WebAssembly and run inside a Web Worker (web/js/worker.js).
//
// The worker sets these globals before starting the program:
//
//	lfSaves   Array<Uint8Array>  saved snapshots, newest first (may be empty)
//	lfOut     (data: string|Uint8Array) => void   message to the game client
//	lfSave    (snapshot: Uint8Array) => void      persist a snapshot
//	lfReady   (info: string) => void              world loaded
//	lfFatal   (message: string) => void           world could not start
//
// and calls these functions, which the program registers:
//
//	lfHello(json), lfSend(json), lfFlush(), lfExport() Uint8Array,
//	lfValidate(Uint8Array) string ("" when valid)
package main

import (
	"syscall/js"
	"time"

	"lanternfall/internal/local"
)

func bytesFromJS(v js.Value) []byte {
	b := make([]byte, v.Get("length").Int())
	js.CopyBytesToGo(b, v)
	return b
}

func bytesToJS(b []byte) js.Value {
	u := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(u, b)
	return u
}

func main() {
	g := js.Global()
	var saves [][]byte
	if arr := g.Get("lfSaves"); arr.Truthy() {
		for i := 0; i < arr.Length(); i++ {
			saves = append(saves, bytesFromJS(arr.Index(i)))
		}
	}
	out := func(text, bin []byte) {
		if bin != nil {
			g.Call("lfOut", bytesToJS(bin))
		} else {
			g.Call("lfOut", string(text))
		}
	}
	save := func(b []byte) { g.Call("lfSave", bytesToJS(b)) }

	h, err := local.Open(saves, out, save)
	if err != nil {
		g.Call("lfFatal", err.Error())
		select {}
	}

	g.Set("lfHello", js.FuncOf(func(_ js.Value, a []js.Value) any { h.Hello([]byte(a[0].String())); return nil }))
	g.Set("lfSend", js.FuncOf(func(_ js.Value, a []js.Value) any { h.Message([]byte(a[0].String())); return nil }))
	g.Set("lfFlush", js.FuncOf(func(_ js.Value, _ []js.Value) any { h.Flush(); return nil }))
	g.Set("lfExport", js.FuncOf(func(_ js.Value, _ []js.Value) any { return bytesToJS(h.Export()) }))
	g.Set("lfValidate", js.FuncOf(func(_ js.Value, a []js.Value) any {
		if err := local.Validate(bytesFromJS(a[0])); err != nil {
			return err.Error()
		}
		return ""
	}))
	g.Call("lfReady", string(h.Info()))

	t := time.NewTicker(50 * time.Millisecond) // 20 Hz, same as the server
	for now := range t.C {
		h.Tick(now)
	}
}
