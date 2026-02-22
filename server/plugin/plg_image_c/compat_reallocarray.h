#ifndef COMPAT_REALLOCARRAY_H
#define COMPAT_REALLOCARRAY_H

#include <stdlib.h>
#include <errno.h>
#include <stdint.h>

/*
 * Provide reallocarray() for platforms that don't have it
 * (e.g. macOS / Apple libc).
 *
 * Matches OpenBSD semantics:
 * - Detects overflow
 * - Sets errno = ENOMEM on overflow
 * - Returns NULL on failure
 */

#if defined(__APPLE__) || !defined(__GLIBC__) || \
    (defined(__GLIBC__) && (__GLIBC__ < 2 || (__GLIBC__ == 2 && __GLIBC_MINOR__ < 26)))

static void *
reallocarray(void *ptr, size_t nmemb, size_t size)
{
    if (size != 0 && nmemb > SIZE_MAX / size)
    {
        errno = ENOMEM;
        return NULL;
    }
    return realloc(ptr, nmemb * size);
}

#endif

#endif /* COMPAT_REALLOCARRAY_H */
