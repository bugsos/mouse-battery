//go:build darwin

#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#include "darwin_cocoa.h"

extern void goDarwinReady(void);
extern void goDarwinMenuClick(uint32_t itemID);

@interface MBMenuEntry : NSObject
@property (nonatomic, assign) uint32_t itemID;
@property (nonatomic, assign) uint8_t kind;
@property (nonatomic, assign) BOOL checked;
@property (nonatomic, copy) NSString *label;
@end

@implementation MBMenuEntry
@end

@interface MBAppDelegate : NSObject <NSApplicationDelegate>
@property (nonatomic, strong) NSStatusItem *statusItem;
- (void)applyTitle:(NSString *)title tooltip:(NSString *)tooltip entries:(NSArray<MBMenuEntry *> *)entries;
- (void)onMenuClick:(NSMenuItem *)sender;
@end

static MBAppDelegate *gDelegate = nil;

@implementation MBAppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    (void)notification;
    self.statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    if (self.statusItem.button != nil) {
        self.statusItem.button.title = @"—";
        self.statusItem.button.toolTip = @"鼠标电量 · 读取中";
    }
    goDarwinReady();
}

- (void)applyTitle:(NSString *)title tooltip:(NSString *)tooltip entries:(NSArray<MBMenuEntry *> *)entries {
    if (self.statusItem == nil) {
        return;
    }
    if (self.statusItem.button != nil) {
        self.statusItem.button.title = title;
        self.statusItem.button.toolTip = tooltip;
    }

    NSMenu *menu = [[NSMenu alloc] initWithTitle:@""];
    menu.autoenablesItems = NO;

    for (MBMenuEntry *entry in entries) {
        if (entry.kind == MB_MENU_SEPARATOR) {
            [menu addItem:[NSMenuItem separatorItem]];
            continue;
        }
        NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:entry.label
                                                      action:nil
                                               keyEquivalent:@""];
        if (entry.kind == MB_MENU_LABEL) {
            item.enabled = NO;
        } else {
            item.enabled = YES;
            item.target = self;
            item.action = @selector(onMenuClick:);
            item.tag = (NSInteger)entry.itemID;
            if (entry.checked) {
                item.state = NSControlStateValueOn;
            }
        }
        [menu addItem:item];
    }

    self.statusItem.menu = menu;
}

- (void)onMenuClick:(NSMenuItem *)sender {
    goDarwinMenuClick((uint32_t)sender.tag);
}

@end

void mb_run(void) {
    @autoreleasepool {
        NSApplication *app = [NSApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
        gDelegate = [[MBAppDelegate alloc] init];
        [app setDelegate:gDelegate];
        [app run];
        gDelegate = nil;
    }
}

void mb_update(const char *title, const char *tooltip, const mb_menu_item *items, int count) {
    @autoreleasepool {
        NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"";
        NSString *nsTooltip = tooltip ? [NSString stringWithUTF8String:tooltip] : @"";
        NSMutableArray<MBMenuEntry *> *entries = [NSMutableArray arrayWithCapacity:(NSUInteger)count];
        for (int i = 0; i < count; i++) {
            MBMenuEntry *e = [[MBMenuEntry alloc] init];
            e.itemID = items[i].id;
            e.kind = items[i].kind;
            e.checked = items[i].checked != 0;
            e.label = items[i].label ? [NSString stringWithUTF8String:items[i].label] : @"";
            [entries addObject:e];
        }
        dispatch_async(dispatch_get_main_queue(), ^{
            [gDelegate applyTitle:nsTitle tooltip:nsTooltip entries:entries];
        });
    }
}

void mb_quit(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp terminate:nil];
    });
}
