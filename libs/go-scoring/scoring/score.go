package scoring

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../lib -lscoring -Wl,-rpath,${SRCDIR}/../lib
#cgo linux,amd64  LDFLAGS: -L${SRCDIR}/../lib -lscoring -Wl,-rpath,${SRCDIR}/../lib
#cgo linux,arm64  LDFLAGS: -L${SRCDIR}/../lib -lscoring -Wl,-rpath,${SRCDIR}/../lib
#include "scoring.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"
)

// ErrNullBuffer is returned when the Rust engine returns a null pointer.
var ErrNullBuffer = errors.New("scoring: engine returned null buffer")

// Score calls the Rust scoring core and returns the result.
// The ctx is checked for cancellation before the (synchronous) FFI call.
func Score(ctx context.Context, fw Framework, responses []Response) (*ScoringResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	type input struct {
		Framework Framework  `json:"framework"`
		Responses []Response `json:"responses"`
	}
	payload, err := json.Marshal(input{Framework: fw, Responses: responses})
	if err != nil {
		return nil, fmt.Errorf("scoring: marshal input: %w", err)
	}

	cInput := C.CString(string(payload))
	defer C.free(unsafe.Pointer(cInput))

	var outLen C.size_t
	buf := C.scoring_score(cInput, C.size_t(len(payload)), &outLen)
	if buf == nil {
		return nil, ErrNullBuffer
	}
	defer C.scoring_free_buffer(buf, outLen)

	// Copy bytes before freeing.
	goBytes := C.GoBytes(unsafe.Pointer(buf), C.int(outLen))

	var env ffiEnvelope
	if err := json.Unmarshal(goBytes, &env); err != nil {
		return nil, fmt.Errorf("scoring: unmarshal envelope: %w", err)
	}

	if !env.OK {
		if env.Error != nil {
			return nil, env.Error
		}
		return nil, errors.New("scoring: engine returned ok=false with no error detail")
	}

	if env.Result == nil {
		return nil, errors.New("scoring: engine returned ok=true with no result")
	}

	return env.Result, nil
}
