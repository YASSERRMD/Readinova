#ifndef SCORING_H
#define SCORING_H

#include <stddef.h>
#include <stdint.h>

/* Score an assessment. input_ptr must be a valid UTF-8 JSON string of
   input_len bytes. Returns a JSON envelope; caller must free with
   scoring_free_buffer. Returns NULL on catastrophic failure. */
extern unsigned char *scoring_score(const char *input_ptr, size_t input_len,
                                    size_t *out_len);

/* Free a buffer returned by scoring_score. */
extern void scoring_free_buffer(unsigned char *ptr, size_t len);

#endif /* SCORING_H */
