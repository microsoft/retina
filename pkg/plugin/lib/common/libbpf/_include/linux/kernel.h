/* SPDX-License-Identifier: (LGPL-2.1 OR BSD-2-Clause) */

#ifndef __LINUX_KERNEL_H
#define __LINUX_KERNEL_H

#include <linux/compiler.h>

#ifndef offsetof
#define offsetof(TYPE, MEMBER) ((size_t) &((TYPE *)0)->MEMBER)
#endif

#ifndef container_of
#define container_of(ptr, type, member) ({			\
	const typeof(((type *)0)->member) * __mptr = (ptr);	\
	(type *)((char *)__mptr - offsetof(type, member)); })
#endif

#ifndef max
#define max(x, y) ({				\
	typeof(x) _max1 = (x);			\
	typeof(y) _max2 = (y);			\
	(void) (&_max1 == &_max2);		\
	_max1 > _max2 ? _max1 : _max2; })
#endif

#ifndef min
#define min(x, y) ({				\
	typeof(x) _min1 = (x);			\
	typeof(y) _min2 = (y);			\
	(void) (&_min1 == &_min2);		\
	_min1 < _min2 ? _min1 : _min2; })
#endif

#ifndef roundup
#define roundup(x, y) (				\
{						\
	const typeof(y) __y = y;		\
	(((x) + (__y - 1)) / __y) * __y;	\
}						\
)
#endif

#define ARRAY_SIZE(arr) (sizeof(arr) / sizeof((arr)[0]))
#define __KERNEL_DIV_ROUND_UP(n, d) (((n) + (d) - 1) / (d))

#if __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
#define be32_to_cpu(x)		__builtin_bswap32(x)
#define cpu_to_be32(x)		__builtin_bswap32(x)
#define be64_to_cpu(x)		__builtin_bswap64(x)
#define cpu_to_be64(x)		__builtin_bswap64(x)
#elif __BYTE_ORDER__ == __ORDER_BIG_ENDIAN__
#define be32_to_cpu(x)		(x)
#define cpu_to_be32(x)		(x)
#define be64_to_cpu(x)		(x)
#define cpu_to_be64(x)		(x)
#else
# error "__BYTE_ORDER__ undefined or invalid"
#endif

#endif
