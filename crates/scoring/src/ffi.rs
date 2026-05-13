//! cdylib entry points for FFI consumers (Go via cgo).
//!
//! The FFI boundary uses JSON: caller passes a JSON-encoded [`FfiInput`];
//! the library returns a JSON-encoded [`FfiEnvelope`].
//!
//! Memory contract:
//! - The returned `*mut u8` buffer is allocated by this library.
//! - The caller **MUST** call [`scoring_free_buffer`] to release it.
//! - The `len` out-parameter receives the byte length (not including a null
//!   terminator).

use std::ffi::c_char;

use serde::{Deserialize, Serialize};

use crate::engine::score;
use crate::types::{Framework, Response, ScoringError, ScoringResult};

#[derive(Deserialize)]
struct FfiInput {
    framework: Framework,
    responses: Vec<Response>,
}

#[derive(Serialize)]
struct FfiEnvelope {
    ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<ScoringResult>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<ScoringError>,
}

/// Score an assessment over the FFI boundary.
///
/// # Safety
/// `input_ptr` must be a valid UTF-8 JSON string of `input_len` bytes.
/// The returned pointer must be freed with [`scoring_free_buffer`].
/// Returns a null pointer on internal panic (caller should treat as an error).
///
/// # Panics
/// Panics are caught internally and surfaced as an [`FfiEnvelope`] error.
#[no_mangle]
pub unsafe extern "C" fn scoring_score(
    input_ptr: *const c_char,
    input_len: usize,
    out_len: *mut usize,
) -> *mut u8 {
    let result = std::panic::catch_unwind(|| {
        // SAFETY: caller guarantees input_ptr is valid for input_len bytes.
        let bytes = unsafe { std::slice::from_raw_parts(input_ptr.cast::<u8>(), input_len) };
        let envelope: FfiEnvelope = match serde_json::from_slice::<FfiInput>(bytes) {
            Ok(input) => match score(&input.framework, &input.responses) {
                Ok(r) => FfiEnvelope {
                    ok: true,
                    result: Some(r),
                    error: None,
                },
                Err(e) => FfiEnvelope {
                    ok: false,
                    result: None,
                    error: Some(e),
                },
            },
            Err(e) => FfiEnvelope {
                ok: false,
                result: None,
                error: Some(ScoringError::FrameworkInvariantViolation {
                    description: format!("JSON parse error: {e}"),
                }),
            },
        };
        serde_json::to_vec(&envelope).unwrap_or_else(|_| b"{}".to_vec())
    });

    let bytes = result.unwrap_or_else(|_| {
        br#"{"ok":false,"error":{"kind":"FrameworkInvariantViolation","description":"panic in scoring engine"}}"#
            .to_vec()
    });

    let len = bytes.len();
    let mut boxed = bytes.into_boxed_slice();
    let ptr = boxed.as_mut_ptr();
    std::mem::forget(boxed);

    // SAFETY: out_len is a valid pointer from the caller.
    unsafe { *out_len = len };
    ptr
}

/// Free a buffer returned by [`scoring_score`].
///
/// # Safety
/// `ptr` must have been returned by [`scoring_score`] and not yet freed.
/// Passing `ptr = null` is a no-op.
#[no_mangle]
pub unsafe extern "C" fn scoring_free_buffer(ptr: *mut u8, len: usize) {
    if !ptr.is_null() {
        // SAFETY: ptr was allocated as a Box<[u8]> of `len` bytes in scoring_score.
        unsafe {
            drop(Box::from_raw(std::ptr::slice_from_raw_parts_mut(ptr, len)));
        }
    }
}
