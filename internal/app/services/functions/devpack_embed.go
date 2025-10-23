package functions

import _ "embed"

// devpackRuntimeSource contains the JavaScript helpers exposed to user functions.
//
//go:embed devpack/runtime.js
var devpackRuntimeSource string
