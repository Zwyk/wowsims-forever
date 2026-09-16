//go:build js && wasm

// A deliberately separate diagnostic WASM entrypoint. Importing core, not sim,
// avoids registering TBC agents, item databases or live combat entrypoints.
package main

import (
	"syscall/js"

	"github.com/wowsims/tbc/sim/core"
)

func main() {
	callback := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"error":"expected one JSON string"}`
		}
		return core.Classic60PreviewJSON(args[0].String())
	})
	js.Global().Set("classic60Preview", callback)
	// Keep the callback registered for the life of this diagnostic module.
	select {}
}
