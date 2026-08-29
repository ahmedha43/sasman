#ifndef SASMAN_FREERADIUS_REGEX_COMPAT_H
#define SASMAN_FREERADIUS_REGEX_COMPAT_H

#include <stdbool.h>
#include <stddef.h>
#include <sys/types.h>
#include <pcre.h>

typedef struct regmatch {
	int a;
	int b;
	int c;
} regmatch_t;

typedef struct regex {
	bool precompiled;
	pcre *compiled;
	pcre_extra *extra;
} regex_t;

#endif
