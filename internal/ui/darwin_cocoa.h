#ifndef MOUSE_BATTERY_DARWIN_COCOA_H
#define MOUSE_BATTERY_DARWIN_COCOA_H

#include <stdint.h>

enum {
    MB_MENU_LABEL = 0,
    MB_MENU_SEPARATOR = 1,
    MB_MENU_RADIO = 2,
    MB_MENU_CHECK = 3,
    MB_MENU_ACTION = 4
};

typedef struct {
    uint32_t id;
    uint8_t kind;
    uint8_t checked;
    const char *label;
} mb_menu_item;

void mb_run(void);
void mb_update(const char *title, const char *tooltip, const mb_menu_item *items, int count);
void mb_quit(void);

#endif
