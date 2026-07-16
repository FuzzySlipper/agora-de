/* agora-de-wayland-input — owned native Wayland input injection engine.
 *
 * Driven by `agora-de-compositorctl input ...` after the bridge has verified
 * the target surface is tracked and input-injectable. Connects as a Wayland
 * client and injects:
 *   - pointer events via zwlr_virtual_pointer_v1  (move / click)
 *   - keyboard events via zwp_virtual_keyboard_v1  (type text / key press)
 *
 * Emits one JSON result object on stdout (success) or stderr (failure) and
 * exits non-zero on failure. There is intentionally no shim or fallback: this
 * helper is the single injection engine.
 *
 * Usage:
 *   agora-de-wayland-input pointer --action move|click [--x N --y N --button BTN --output-w W --output-h H]
 *   agora-de-wayland-input keyboard --action type --text "STRING"
 *   agora-de-wayland-input keyboard --action key   --keysym 65293   # or --code 28 (evdev scancode)
 */
#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <unistd.h>
#include <poll.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>
#include <xkbcommon/xkbcommon-keysyms.h>
#include "wlr-virtual-pointer-client-protocol.h"
#include "virtual-keyboard-client-protocol.h"
#include "input-method-client-protocol.h"

struct state {
    struct wl_seat *seat;
    struct zwlr_virtual_pointer_manager_v1 *vp_mgr;
    struct zwp_virtual_keyboard_manager_v1 *vk_mgr;
    const char *device; /* "pointer" or "keyboard": only bind the manager we need */
    /* compositor-provided keymap (copied to our own memfd) */
    int keymap_fd;
    uint32_t keymap_size;
    uint32_t keymap_format;
    int have_keymap;
};

static void registry_global(void *data, struct wl_registry *reg, uint32_t name,
                            const char *iface, uint32_t ver) {
    struct state *s = data;
    if (strcmp(iface, "wl_seat") == 0) {
        uint32_t seat_ver = ver < 7 ? ver : 7;
        s->seat = wl_registry_bind(reg, name, &wl_seat_interface, seat_ver);
    } else if (s->device && strcmp(s->device, "pointer") == 0 && strcmp(iface, "zwlr_virtual_pointer_manager_v1") == 0) {
        s->vp_mgr = wl_registry_bind(reg, name, &zwlr_virtual_pointer_manager_v1_interface, 1);
    } else if (s->device && strcmp(s->device, "keyboard") == 0 && strcmp(iface, "zwp_virtual_keyboard_manager_v1") == 0) {
        s->vk_mgr = wl_registry_bind(reg, name, &zwp_virtual_keyboard_manager_v1_interface, 1);
    }
}
static void registry_global_remove(void *data, struct wl_registry *reg, uint32_t name) {
    (void)data; (void)reg; (void)name;
}
static const struct wl_registry_listener registry_listener = {
    .global = registry_global,
    .global_remove = registry_global_remove,
};

static void keyboard_keymap(void *data, struct wl_keyboard *kb, uint32_t format, int32_t fd, uint32_t size) {
    (void)kb;
    struct state *s = data;
    if (size == 0) { close(fd); return; }
    void *map = mmap(NULL, size, PROT_READ, MAP_PRIVATE, fd, 0);
    if (map == MAP_FAILED) { close(fd); return; }
    int mem = memfd_create("agora-de-xkb-keymap", MFD_CLOEXEC);
    if (mem < 0 || write(mem, map, size) != (ssize_t)size) {
        if (mem >= 0) close(mem);
        munmap(map, size); close(fd); return;
    }
    lseek(mem, 0, SEEK_SET);
    munmap(map, size); close(fd);
    if (s->have_keymap && s->keymap_fd >= 0) close(s->keymap_fd);
    s->keymap_fd = mem;
    s->keymap_size = size;
    s->keymap_format = format;
    s->have_keymap = 1;
}
static void keyboard_other(void *data, struct wl_keyboard *kb) { (void)data; (void)kb; }
static const struct wl_keyboard_listener keyboard_listener = {
    .keymap = keyboard_keymap,
    .enter = (void (*)(void *, struct wl_keyboard *, uint32_t, struct wl_surface *, struct wl_array *))keyboard_other,
    .leave = (void (*)(void *, struct wl_keyboard *, uint32_t, struct wl_surface *))keyboard_other,
    .key = (void (*)(void *, struct wl_keyboard *, uint32_t, uint32_t, uint32_t, uint32_t))keyboard_other,
    .modifiers = (void (*)(void *, struct wl_keyboard *, uint32_t, uint32_t, uint32_t, uint32_t, uint32_t))keyboard_other,
    .repeat_info = (void (*)(void *, struct wl_keyboard *, int32_t, int32_t))keyboard_other,
};

static void emit_ok_obj(const char *s) { printf("%s\n", s); fflush(stdout); }
static void emit_err(const char *msg) { fprintf(stderr, "{\"ok\":false,\"error\":\"%s\"}\n", msg); fflush(stderr); }

/* ---------------- pointer ---------------- */

static int run_pointer(struct state *s, struct wl_display *disp, int argc, char **argv) {
    const char *action = "click";
    int x = 0, y = 0, w = 2560, h = 1440;
    uint32_t btn = 0x110; /* BTN_LEFT */
    for (int i = 0; i < argc; i++) {
        if (!strcmp(argv[i], "--action") && i + 1 < argc) action = argv[++i];
        else if (!strcmp(argv[i], "--x") && i + 1 < argc) x = atoi(argv[++i]);
        else if (!strcmp(argv[i], "--y") && i + 1 < argc) y = atoi(argv[++i]);
        else if (!strcmp(argv[i], "--button") && i + 1 < argc) btn = (uint32_t)strtoul(argv[++i], NULL, 0);
        else if (!strcmp(argv[i], "--output-w") && i + 1 < argc) w = atoi(argv[++i]);
        else if (!strcmp(argv[i], "--output-h") && i + 1 < argc) h = atoi(argv[++i]);
    }
    if (!s->vp_mgr) { emit_err("no zwlr_virtual_pointer_manager_v1"); return 5; }
    struct zwlr_virtual_pointer_v1 *vp =
        zwlr_virtual_pointer_manager_v1_create_virtual_pointer(s->vp_mgr, s->seat);
    if (!vp) { emit_err("create_virtual_pointer failed"); return 6; }
    zwlr_virtual_pointer_v1_motion_absolute(vp, 0, x, y, (uint32_t)w, (uint32_t)h);
    zwlr_virtual_pointer_v1_frame(vp);
    if (!strcmp(action, "click")) {
        zwlr_virtual_pointer_v1_button(vp, 0, btn, WL_POINTER_BUTTON_STATE_PRESSED);
        zwlr_virtual_pointer_v1_frame(vp);
        zwlr_virtual_pointer_v1_button(vp, 0, btn, WL_POINTER_BUTTON_STATE_RELEASED);
        zwlr_virtual_pointer_v1_frame(vp);
    }
    wl_display_flush(disp);
    usleep(20000);
    wl_display_roundtrip(disp);
    zwlr_virtual_pointer_v1_destroy(vp);
    wl_display_roundtrip(disp);
    char out[160];
    snprintf(out, sizeof(out), "{\"ok\":true,\"device\":\"pointer\",\"action\":\"%s\",\"x\":%d,\"y\":%d,\"button\":%u}",
             action, x, y, btn);
    emit_ok_obj(out);
    return 0;
}

/* ---------------- keyboard ---------------- */

static int find_keycode_unused_removed(struct xkb_keymap *km, xkb_keysym_t sym, xkb_keycode_t *out_kc, int *out_level) {
    (void)km; (void)sym; (void)out_kc; (void)out_level; return -1;
}

/* decode one UTF-8 codepoint starting at *i in text; advance *i; return codepoint or -1 on end, -2 on error */
static int32_t utf8_next(const char *text, size_t len, size_t *i) {
    if (*i >= len) return -1;
    unsigned char c = (unsigned char)text[*i];
    uint32_t cp; int extra;
    if (c < 0x80) { cp = c; extra = 0; }
    else if ((c & 0xE0) == 0xC0) { cp = c & 0x1F; extra = 1; }
    else if ((c & 0xF0) == 0xE0) { cp = c & 0x0F; extra = 2; }
    else if ((c & 0xF8) == 0xF0) { cp = c & 0x07; extra = 3; }
    else { (*i)++; return -2; }
    for (int k = 0; k < extra; k++) {
        if (*i + 1 + k >= len) { *i = len; return -2; }
        unsigned char cc = (unsigned char)text[*i + 1 + k];
        if ((cc & 0xC0) != 0x80) { (*i)++; return -2; }
        cp = (cp << 6) | (cc & 0x3F);
    }
    *i += 1 + extra;
    return (int32_t)cp;
}

static int run_keyboard(struct state *s, struct wl_display *disp, int argc, char **argv) {
    const char *action = NULL;
    const char *text = NULL;
    unsigned long keysym = 0;
    for (int i = 0; i < argc; i++) {
        if (!strcmp(argv[i], "--action") && i + 1 < argc) action = argv[++i];
        else if (!strcmp(argv[i], "--text") && i + 1 < argc) text = argv[++i];
        else if (!strcmp(argv[i], "--keysym") && i + 1 < argc) keysym = strtoul(argv[++i], NULL, 0);
    }
    if (!action) { emit_err("keyboard --action is required: type|key"); return 2; }
    if (!s->vk_mgr) { emit_err("no zwp_virtual_keyboard_manager_v1"); return 5; }

    /* Build the list of keysyms to emit. For `type`, each UTF-8 char becomes a
     * keysym; for `key`, a single keysym. */
    xkb_keysym_t wanted_syms[1024];
    size_t wanted = 0;
    if (!strcmp(action, "type")) {
        if (!text) { emit_err("keyboard type requires --text"); return 2; }
        size_t len = strlen(text), i = 0; int32_t cp;
        while ((cp = utf8_next(text, len, &i)) >= 0 && wanted < 1024) {
            xkb_keysym_t sym = xkb_utf32_to_keysym((uint32_t)cp);
            wanted_syms[wanted++] = (sym == XKB_KEY_NoSymbol) ? 0 : sym;
        }
    } else if (!strcmp(action, "key")) {
        wanted_syms[wanted++] = (xkb_keysym_t)keysym;
    } else {
        emit_err("unknown keyboard action (type|key)");
        return 2;
    }
    if (wanted == 0) { emit_err("no keysyms to type"); return 2; }

    /* Assign each DISTINCT keysym a fresh keycode (wtype-style). The virtual
     * keyboard keymap maps keycode (8 + index + 1) -> keysym directly, so no
     * shift/modifier dance is needed: each keycode emits the exact keysym and
     * the client produces the target character. This custom-keycode form is the
     * one Wayfire's virtual keyboard accepts; evdev-based keymaps are rejected. */
    xkb_keysym_t distinct[1024]; int idx[1024]; size_t ndistinct = 0;
    for (size_t i = 0; i < wanted; i++) {
        xkb_keysym_t sym = wanted_syms[i];
        ssize_t found = -1;
        for (size_t j = 0; j < ndistinct; j++) if (distinct[j] == sym) { found = (ssize_t)j; break; }
        if (found < 0) { distinct[ndistinct] = sym; found = (ssize_t)ndistinct; ndistinct++; }
        idx[i] = (int)found;
    }

    char *kmbuf = NULL; size_t kmsize = 0;
    FILE *kmf = open_memstream(&kmbuf, &kmsize);
    if (!kmf) { emit_err("open_memstream failed"); return 7; }
    fprintf(kmf, "xkb_keymap {\n");
    fprintf(kmf, "xkb_keycodes \"(unnamed)\" {\nminimum = 8;\nmaximum = %zu;\n", ndistinct + 8 + 1);
    for (size_t i = 0; i < ndistinct; i++) fprintf(kmf, "<K%zu> = %zu;\n", i + 1, i + 8 + 1);
    fprintf(kmf, "};\n");
    fprintf(kmf, "xkb_types \"(unnamed)\" { include \"complete\" };\n");
    fprintf(kmf, "xkb_compatibility \"(unnamed)\" { include \"complete\" };\n");
    fprintf(kmf, "xkb_symbols \"(unnamed)\" {\n");
    for (size_t i = 0; i < ndistinct; i++) {
        char name[64];
        if (xkb_keysym_get_name(distinct[i], name, sizeof(name)) <= 0) snprintf(name, sizeof(name), "0x%x", (unsigned)distinct[i]);
        fprintf(kmf, "key <K%zu> {[%s]};\n", i + 1, name);
    }
    fprintf(kmf, "};\n};\n");
    fputc('\0', kmf);
    fflush(kmf);
    fclose(kmf);

    char tmpfile[] = "/tmp/agora-de-km-XXXXXX";
    int keymap_fd = mkstemp(tmpfile);
    if (keymap_fd < 0) { free(kmbuf); emit_err("mkstemp failed"); return 7; }
    unlink(tmpfile);
    if (write(keymap_fd, kmbuf, kmsize) != (ssize_t)kmsize) { free(kmbuf); emit_err("keymap write failed"); return 7; }
    lseek(keymap_fd, 0, SEEK_SET);

    struct zwp_virtual_keyboard_v1 *vk =
        zwp_virtual_keyboard_manager_v1_create_virtual_keyboard(s->vk_mgr, s->seat);
    if (!vk) { free(kmbuf); emit_err("create_virtual_keyboard failed"); return 6; }
    zwp_virtual_keyboard_v1_keymap(vk, 1 /* xkb_v1 */, keymap_fd, (uint32_t)kmsize);

    uint32_t time = 0; int typed = 0, skipped = 0;
    for (size_t i = 0; i < wanted; i++) {
        if (distinct[idx[i]] == 0) { skipped++; continue; }
        uint32_t scancode = (uint32_t)(idx[i] + 1);
        zwp_virtual_keyboard_v1_key(vk, time++, scancode, WL_KEYBOARD_KEY_STATE_PRESSED);
        zwp_virtual_keyboard_v1_key(vk, time++, scancode, WL_KEYBOARD_KEY_STATE_RELEASED);
        typed++;
    }
    zwp_virtual_keyboard_v1_destroy(vk);
    wl_display_flush(disp);
    usleep(20000);
    wl_display_roundtrip(disp);
    free(kmbuf);

    char out[256];
    snprintf(out, sizeof(out), "{\"ok\":true,\"device\":\"keyboard\",\"action\":\"%s\",\"typed\":%d,\"skipped\":%d}",
             action, typed, skipped);
    emit_ok_obj(out);
    return 0;
}

/* ---------------- input-method (text-input-v3 commit) ---------------- */

static struct zwp_input_method_v1 *im_obj;
static struct zwp_input_method_context_v1 *im_ctx;
static uint32_t im_serial;
static int im_have_serial;
static const char *im_text;

static const struct zwp_input_method_context_v1_listener im_ctx_listener;

static void im_activate(void *data, struct zwp_input_method_v1 *ime, struct zwp_input_method_context_v1 *id) {
    (void)data; (void)ime;
    if (im_ctx) zwp_input_method_context_v1_destroy(im_ctx);
    im_ctx = id;
    zwp_input_method_context_v1_add_listener(im_ctx, &im_ctx_listener, NULL);
}
static void im_deactivate(void *data, struct zwp_input_method_v1 *ime, struct zwp_input_method_context_v1 *c) {
    (void)data; (void)ime;
    if (c && c == im_ctx) { zwp_input_method_context_v1_destroy(im_ctx); im_ctx = NULL; }
}
static const struct zwp_input_method_v1_listener im_listener = {
    .activate = im_activate,
    .deactivate = im_deactivate,
};
static void im_ctx_surrounding(void *d, struct zwp_input_method_context_v1 *c, const char *t, uint32_t cur, uint32_t anc) { (void)d;(void)c;(void)t;(void)cur;(void)anc; }
static void im_ctx_reset(void *d, struct zwp_input_method_context_v1 *c) { (void)d;(void)c; }
static void im_ctx_content_type(void *d, struct zwp_input_method_context_v1 *c, uint32_t h, uint32_t p) { (void)d;(void)c;(void)h;(void)p; }
static void im_ctx_invoke_action(void *d, struct zwp_input_method_context_v1 *c, uint32_t b, uint32_t i) { (void)d;(void)c;(void)b;(void)i; }
static void im_ctx_commit_state(void *d, struct zwp_input_method_context_v1 *c, uint32_t serial) { (void)d;(void)c; im_serial = serial; im_have_serial = 1; }
static void im_ctx_preferred_language(void *d, struct zwp_input_method_context_v1 *c, const char *l) { (void)d;(void)c;(void)l; }
static const struct zwp_input_method_context_v1_listener im_ctx_listener = {
    .surrounding_text = im_ctx_surrounding,
    .reset = im_ctx_reset,
    .content_type = im_ctx_content_type,
    .invoke_action = im_ctx_invoke_action,
    .commit_state = im_ctx_commit_state,
    .preferred_language = im_ctx_preferred_language,
};

static void im_reg_global(void *data, struct wl_registry *reg, uint32_t name, const char *iface, uint32_t ver) {
    (void)data;
    if (!strcmp(iface, "zwp_input_method_v1"))
        im_obj = wl_registry_bind(reg, name, &zwp_input_method_v1_interface, ver < 1 ? 1 : ver);
}
static void im_reg_remove(void *data, struct wl_registry *reg, uint32_t name) { (void)data;(void)reg;(void)name; }
static const struct wl_registry_listener im_reg_listener = { .global = im_reg_global, .global_remove = im_reg_remove };

/* Commit text into the focused text-input-v3 client (e.g. Chromium). Requires
 * a text-input-v3 input to be focused so the compositor activates this input
 * method. Usage: input-method --text "STRING" [--timeout-ms N] */
static int run_input_method(struct wl_display *disp, int argc, char **argv) {
    im_text = "";
    int timeout_ms = 5000;
    for (int i = 0; i < argc; i++) {
        if (!strcmp(argv[i], "--text") && i + 1 < argc) im_text = argv[++i];
        else if (!strcmp(argv[i], "--timeout-ms") && i + 1 < argc) timeout_ms = atoi(argv[++i]);
    }
    struct wl_registry *reg = wl_display_get_registry(disp);
    wl_registry_add_listener(reg, &im_reg_listener, NULL);
    wl_display_roundtrip(disp);
    if (!im_obj) { emit_err("no zwp_input_method_v1 global (enable Wayfire input-method-v1 plugin)"); return 5; }
    zwp_input_method_v1_add_listener(im_obj, &im_listener, NULL);

    struct pollfd pfd = { wl_display_get_fd(disp), POLLIN, 0 };
    int waited = 0;
    while (!im_ctx && waited < timeout_ms) {
        while (wl_display_prepare_read(disp) != 0) wl_display_dispatch_pending(disp);
        wl_display_flush(disp);
        if (poll(&pfd, 1, 50) < 0) break;
        wl_display_read_events(disp);
        wl_display_dispatch_pending(disp);
        waited += 50;
    }
    if (!im_ctx) { emit_err("no activate (is a text-input-v3 input focused?)"); return 4; }
    wl_display_dispatch_pending(disp);
    uint32_t serial = im_have_serial ? im_serial : 0;
    zwp_input_method_context_v1_commit_string(im_ctx, serial, im_text);
    wl_display_flush(disp);
    usleep(100000);
    wl_display_roundtrip(disp);
    zwp_input_method_context_v1_destroy(im_ctx);
    wl_display_roundtrip(disp);
    char out[512];
    snprintf(out, sizeof(out), "{\"ok\":true,\"device\":\"input-method\",\"committed\":\"%s\",\"serial\":%u}", im_text, serial);
    emit_ok_obj(out);
    return 0;
}

int main(int argc, char **argv) {
    if (argc < 2) { emit_err("usage: <pointer|keyboard> ..."); return 2; }

    struct wl_display *disp = wl_display_connect(NULL);
    if (!disp) { emit_err("wayland connect failed"); return 3; }

    struct state s; memset(&s, 0, sizeof(s));
    s.device = argv[1];
    struct wl_registry *reg = wl_display_get_registry(disp);
    wl_registry_add_listener(reg, &registry_listener, &s);
    wl_display_roundtrip(disp);

    if (!s.seat) { emit_err("no wl_seat"); return 4; }

    int rc;
    if (!strcmp(argv[1], "pointer")) {
        rc = run_pointer(&s, disp, argc - 2, argv + 2);
    } else if (!strcmp(argv[1], "keyboard")) {
        rc = run_keyboard(&s, disp, argc - 2, argv + 2);
    } else if (!strcmp(argv[1], "input-method")) {
        rc = run_input_method(disp, argc - 2, argv + 2);
    } else {
        emit_err("unknown device (pointer|keyboard|input-method)");
        rc = 2;
    }
    wl_display_disconnect(disp);
    return rc;
}
