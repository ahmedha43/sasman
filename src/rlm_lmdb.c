#define NEVER_RCSID
#include <freeradius/build.h>
#include <freeradius/radiusd.h>
#include <freeradius/modules.h>
#include <freeradius/libradius.h>
#include <lmdb.h>
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <sys/stat.h>
#include <netinet/in.h>
#include <arpa/inet.h>

typedef struct rlm_lmdb_t {
    char const *db_dir;
    char const *db_name;
    uint32_t   map_size;
    MDB_env    *env;
} rlm_lmdb_t;

static const CONF_PARSER module_config[] = {
    { "db_dir",   PW_TYPE_STRING | PW_TYPE_NOT_EMPTY, offsetof(rlm_lmdb_t, db_dir),   NULL, "/app/data/radius_db" },
    { "db_name",  PW_TYPE_STRING | PW_TYPE_NOT_EMPTY, offsetof(rlm_lmdb_t, db_name),  NULL, "sasman.mdb" },
    { "map_size", PW_TYPE_INTEGER,                   offsetof(rlm_lmdb_t, map_size), NULL, "104857600" }, /* 100MB */
    CONF_PARSER_TERMINATOR
};

static int mod_instantiate(CONF_SECTION *conf, void *instance) {
    rlm_lmdb_t *inst = instance;
    int rc;

    if (!inst->db_dir || inst->db_dir[0] == '\0') {
        fprintf(stderr, "rlm_lmdb: db_dir is empty\n");
        return -1;
    }

    if (mkdir(inst->db_dir, 0775) != 0 && errno != EEXIST) {
        fprintf(stderr, "rlm_lmdb: failed to create db_dir %s: %s\n", inst->db_dir, strerror(errno));
        return -1;
    }

    rc = mdb_env_create(&inst->env);
    if (rc != 0) {
        fprintf(stderr, "rlm_lmdb: mdb_env_create failed: %s\n", mdb_strerror(rc));
        return -1;
    }

    rc = mdb_env_set_mapsize(inst->env, inst->map_size);
    if (rc != 0) {
        fprintf(stderr, "rlm_lmdb: mdb_env_set_mapsize failed: %s\n", mdb_strerror(rc));
        return -1;
    }

    rc = mdb_env_set_maxdbs(inst->env, 10);
    if (rc != 0) {
        fprintf(stderr, "rlm_lmdb: mdb_env_set_maxdbs failed: %s\n", mdb_strerror(rc));
        return -1;
    }

    mode_t old_umask = umask(0000);
    rc = mdb_env_open(inst->env, inst->db_dir, 0, 0666);
    umask(old_umask);
    if (rc != 0) {
        fprintf(stderr, "rlm_lmdb: mdb_env_open %s failed: %s\n", inst->db_dir, mdb_strerror(rc));
        return -1;
    }

    return 0;
}

static int mod_detach(void *instance) {
    rlm_lmdb_t *inst = instance;
    if (inst->env) mdb_env_close(inst->env);
    return 0;
}

static rlm_rcode_t mod_authorize(void *instance, REQUEST *request) {
    rlm_lmdb_t *inst = instance;
    MDB_txn *txn;
    MDB_dbi dbi_users, dbi_sessions;
    MDB_val key, data;
    int rc;
    time_t now = time(NULL);

    if (!request->username) return RLM_MODULE_NOOP;

    rc = mdb_txn_begin(inst->env, NULL, MDB_RDONLY, &txn);
    if (rc != 0) return RLM_MODULE_FAIL;

    /* 1. Check for Active Session (Informational Only) */
    rc = mdb_dbi_open(txn, "sessions", 0, &dbi_sessions);
    if (rc == MDB_SUCCESS) {
        key.mv_size = strlen(request->username->vp_strvalue);
        key.mv_data = (void *)request->username->vp_strvalue;
        if (mdb_get(txn, dbi_sessions, &key, &data) == MDB_SUCCESS) {
            /* 
             * Always allow. The new Accounting-Start will simply overwrite the old session in LMDB.
             */
            RDEBUG2("rlm_lmdb: User %s has an existing session. Allowing re-login (automatic).", request->username->vp_strvalue);
        }
    }

    /* 2. Get User Credentials & Attributes */
    rc = mdb_dbi_open(txn, "radcheck", 0, &dbi_users);
    if (rc != 0) { mdb_txn_abort(txn); return RLM_MODULE_NOOP; }

    key.mv_size = strlen(request->username->vp_strvalue);
    key.mv_data = (void *)request->username->vp_strvalue;

    rc = mdb_get(txn, dbi_users, &key, &data);
    if (rc == MDB_SUCCESS) {
        char *copy = rad_malloc(data.mv_size + 1);
        memcpy(copy, data.mv_data, data.mv_size);
        copy[data.mv_size] = '\0';

        char *line = strtok(copy, "\n");
        int line_idx = 0;
        while (line != NULL) {
            if (line_idx == 0) {
                // First line is always the password
                fr_pair_make(request, &request->config, "Cleartext-Password", line, T_OP_SET);
            } else {
                char *eq = strchr(line, '=');
                if (eq) {
                    *eq = '\0';
                    char *k = line;
                    char *v = eq + 1;

                    if (strcmp(k, "Enabled") == 0) {
                        if (atoi(v) == 0) {
                            REJECT("rlm_lmdb: User %s is disabled", request->username->vp_strvalue);
                            free(copy);
                            mdb_txn_abort(txn);
                            return RLM_MODULE_REJECT;
                        }
                    } else if (strcmp(k, "Expiration") == 0) {
                        time_t exp = (time_t)atoll(v);
                        if (exp > 0 && now >= exp) {
                            REJECT("rlm_lmdb: User %s has expired (at %s)", request->username->vp_strvalue, v);
                            free(copy);
                            mdb_txn_abort(txn);
                            return RLM_MODULE_REJECT;
                        }
                    } else {
                        // Standard reply attributes
                        fr_pair_make(request->reply, &request->reply->vps, k, v, T_OP_SET);
                    }
                }
            }
            line = strtok(NULL, "\n");
            line_idx++;
        }
        free(copy);
        mdb_txn_commit(txn);
        return RLM_MODULE_OK;
    }

    mdb_txn_abort(txn);
    return RLM_MODULE_NOTFOUND;
}

static rlm_rcode_t mod_accounting(void *instance, REQUEST *request) {
    rlm_lmdb_t *inst = instance;
    MDB_txn *txn;
    MDB_dbi dbi_sessions, dbi_bw;
    MDB_val key, data;
    int rc;
    time_t now = time(NULL);

    VALUE_PAIR *status = fr_pair_find_by_num(request->packet->vps, PW_ACCT_STATUS_TYPE, 0, TAG_ANY);
    if (!status || !request->username) return RLM_MODULE_NOOP;

    /* Only handle Start, Interim-Update (Alive), and Stop */
    uint32_t status_val = status->vp_integer;
    if (status_val != PW_STATUS_START &&
        status_val != PW_STATUS_ALIVE &&
        status_val != PW_STATUS_STOP) {
        return RLM_MODULE_NOOP;
    }

    rc = mdb_txn_begin(inst->env, NULL, 0, &txn);
    if (rc != 0) return RLM_MODULE_FAIL;

    rc = mdb_dbi_open(txn, "sessions",   MDB_CREATE, &dbi_sessions);
    if (rc != 0) { mdb_txn_abort(txn); return RLM_MODULE_FAIL; }
    rc = mdb_dbi_open(txn, "bandwidth",  MDB_CREATE, &dbi_bw);
    if (rc != 0) { mdb_txn_abort(txn); return RLM_MODULE_FAIL; }

    key.mv_size = strlen(request->username->vp_strvalue);
    key.mv_data = (void *)request->username->vp_strvalue;

    /* Grab common attributes */
    VALUE_PAIR *sid    = fr_pair_find_by_num(request->packet->vps, PW_ACCT_SESSION_ID,    0, TAG_ANY);
    VALUE_PAIR *ip     = fr_pair_find_by_num(request->packet->vps, PW_FRAMED_IP_ADDRESS,  0, TAG_ANY);
    VALUE_PAIR *cli    = fr_pair_find_by_num(request->packet->vps, PW_CALLING_STATION_ID, 0, TAG_ANY);
    VALUE_PAIR *in_oct = fr_pair_find_by_num(request->packet->vps, PW_ACCT_INPUT_OCTETS,  0, TAG_ANY);
    VALUE_PAIR *out_oct= fr_pair_find_by_num(request->packet->vps, PW_ACCT_OUTPUT_OCTETS, 0, TAG_ANY);
    VALUE_PAIR *in_gw  = fr_pair_find_by_num(request->packet->vps, 52 /* Input-Gigawords  */, 0, TAG_ANY);
    VALUE_PAIR *out_gw = fr_pair_find_by_num(request->packet->vps, 53 /* Output-Gigawords */, 0, TAG_ANY);
    VALUE_PAIR *sess_t = fr_pair_find_by_num(request->packet->vps, PW_ACCT_SESSION_TIME,  0, TAG_ANY);

    /* Compute total bytes (octets + gigawords for >4GB sessions) */
    uint64_t bytes_in  = in_oct  ? (uint64_t)in_oct->vp_integer  : 0;
    uint64_t bytes_out = out_oct ? (uint64_t)out_oct->vp_integer : 0;
    if (in_gw)  bytes_in  += (uint64_t)in_gw->vp_integer  << 32;
    if (out_gw) bytes_out += (uint64_t)out_gw->vp_integer << 32;
    uint32_t sess_secs = sess_t ? sess_t->vp_integer : 0;

    char ip_str[64]  = "";
    char sid_str[64] = "";
    char cli_str[128]= "";
    if (ip)  snprintf(ip_str,  sizeof(ip_str),  "%s", inet_ntoa(*(struct in_addr *)&ip->vp_ipaddr));
    if (sid) snprintf(sid_str, sizeof(sid_str), "%s", sid->vp_strvalue);
    if (cli) snprintf(cli_str, sizeof(cli_str), "%s", cli->vp_strvalue);

    char val[512];
    int  len;

    if (status_val == PW_STATUS_START) {
        /* sessions DB: sessionId|ip|callingStation */
        len = snprintf(val, sizeof(val), "%s|%s|%s", sid_str, ip_str, cli_str);
        data.mv_size = len; data.mv_data = val;
        mdb_put(txn, dbi_sessions, &key, &data, 0);

        /* bandwidth DB: inBytes|outBytes|startTime|sessionSecs|sessionId|ip|cli */
        len = snprintf(val, sizeof(val), "0|0|%ld|0|%s|%s|%s",
                       (long)now, sid_str, ip_str, cli_str);
        data.mv_size = len; data.mv_data = val;
        mdb_put(txn, dbi_bw, &key, &data, 0);
        
        RDEBUG2("rlm_lmdb: Session START for user %s (SID: %s)", (char *)key.mv_data, sid_str);

    } else if (status_val == PW_STATUS_ALIVE) {
        /* Recover stored start_time from existing bandwidth record */
        long start_time = (long)(now - (long)sess_secs);
        MDB_val existing;
        if (mdb_get(txn, dbi_bw, &key, &existing) == MDB_SUCCESS) {
            char *copy = (char *)rad_malloc(existing.mv_size + 1);
            memcpy(copy, existing.mv_data, existing.mv_size);
            copy[existing.mv_size] = '\0';
            char *tok = strtok(copy, "|");
            if (tok) tok = strtok(NULL, "|");
            if (tok) tok = strtok(NULL, "|");
            if (tok) start_time = atol(tok);
            free(copy);
        }

        /* Update bandwidth DB with latest counters */
        len = snprintf(val, sizeof(val), "%llu|%llu|%ld|%u|%s|%s|%s",
                       (unsigned long long)bytes_in,
                       (unsigned long long)bytes_out,
                       start_time,
                       (unsigned int)sess_secs,
                       sid_str, ip_str, cli_str);
        data.mv_size = len; data.mv_data = val;
        mdb_put(txn, dbi_bw, &key, &data, 0);

        len = snprintf(val, sizeof(val), "%s|%s|%s", sid_str, ip_str, cli_str);
        data.mv_size = len; data.mv_data = val;
        mdb_put(txn, dbi_sessions, &key, &data, 0);
        
        RDEBUG3("rlm_lmdb: Session ALIVE update for user %s (In: %llu, Out: %llu)", (char *)key.mv_data, bytes_in, bytes_out);

    } else { /* PW_STATUS_STOP */
        int r1 = mdb_del(txn, dbi_sessions, &key, NULL);
        int r2 = mdb_del(txn, dbi_bw,       &key, NULL);
        RDEBUG2("rlm_lmdb: Session STOP for user %s (Deleted: %s)", (char *)key.mv_data, (r1 == 0) ? "yes" : "no (wasn't there)");
    }

    mdb_txn_commit(txn);
    return RLM_MODULE_OK;
}

extern module_t rlm_lmdb;
module_t rlm_lmdb = {
    .magic        = RLM_MODULE_INIT,
    .name         = "lmdb",
    .type         = RLM_TYPE_THREAD_SAFE,
    .inst_size    = sizeof(rlm_lmdb_t),
    .config       = module_config,
    .instantiate  = mod_instantiate,
    .detach       = mod_detach,
    .methods = {
        [MOD_AUTHORIZE]   = mod_authorize,
        [MOD_ACCOUNTING]  = mod_accounting,
    },
};
