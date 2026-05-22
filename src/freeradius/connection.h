#ifndef SASMAN_FREERADIUS_CONNECTION_COMPAT_H
#define SASMAN_FREERADIUS_CONNECTION_COMPAT_H

typedef struct rad_listen_t rad_listen_t;
typedef struct RADCLIENT RADCLIENT;
typedef struct RADCLIENT_LIST RADCLIENT_LIST;
typedef enum rad_listen_type_t {
	RAD_LISTEN_NONE = 0,
	RAD_LISTEN_MAX = 8
} RAD_LISTEN_TYPE;

typedef int (*RAD_REQUEST_FUNP)(void *);
typedef int (*fr_request_process_t)(void *);
typedef int fr_state_action_t;
typedef struct fr_event_t fr_event_t;
#endif
