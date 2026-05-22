#ifndef SASMAN_FREERADIUS_LOG_COMPAT_H
#define SASMAN_FREERADIUS_LOG_COMPAT_H

#include <stdarg.h>

typedef int log_lvl_t;
typedef void (*radlog_func_t)(int, char const *, va_list);

#define REDEBUG(...) module_failure_msg(request, __VA_ARGS__)
#define RDEBUG(...)  do { } while (0)
#define RDEBUG2(...) do { } while (0)
#define RDEBUG3(...) do { } while (0)
#define RDEBUG4(...) do { } while (0)

#ifndef REJECT
#define REJECT(...) module_failure_msg(request, __VA_ARGS__)
#endif

#endif
