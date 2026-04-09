package setay

import "os"

// variableResolver is the package-level custom variable resolver.
// When nil, only environment variables are consulted.
var variableResolver func(string) (string, bool, error)

// RegisterVariableResolver registers a custom variable resolver function.
// The function receives a variable name and must return:
//   - result: the resolved value (used only when ok is true)
//   - ok: true if the variable was resolved, false to fall through to os.LookupEnv
//   - err: a non-nil error causes Unmarshal to return immediately with that error
//
// Call RegisterVariableResolver(nil) to reset to the default behavior
// (environment variables only).
func RegisterVariableResolver(fn func(string) (string, bool, error)) {
	variableResolver = fn
}

// resolveVar resolves a variable name using the following priority:
//  1. Custom resolver (if registered and returns ok=true)
//  2. Environment variable via os.LookupEnv (if resolver is nil or ok=false)
//
// Returns (value, found, err).
// found=false means the variable was not resolved by any source.
func resolveVar(name string) (string, bool, error) {
	if variableResolver != nil {
		val, ok, err := variableResolver(name)
		if err != nil {
			return "", false, err
		}
		if ok {
			return val, true, nil
		}
		// ok=false: fall through to environment variable
	}
	val, ok := os.LookupEnv(name)
	return val, ok, nil
}
