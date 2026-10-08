# Design

`Step` in `internal/core` is a pure function. It takes a config, the current state, and one observation, and it returns the next state plus a list of actions. `Advance` runs those actions and keeps an effect only when the action succeeds. The observation carries the power source, the HID idle duration when it is known, whether this process currently holds the sleep assertion, and whether a notification was delivered since the last poll. The power source is `AC`, `Battery`, or `Unknown`. The state records two facts. `WakeOwed` means a notification still needs `DeclareActivity`. `Display` is the reducer's belief, `On` or `Off`.

The assertion is held exactly while power is `AC`. `Unknown` releases it. Whether it is held is read from the host after each action, not stored on `State`. A `caffeinate` process that has already exited is acquired again. The display belief stays `On` until `TurnDisplayOff` succeeds, and that success is what stops the next idle poll from emitting the action again. The belief is not the panel reading. `AppleCLCD2` can fail, and a failed read must not look like `On` and turn the panel off on every poll. The reducer assumes `DeclareActivity` resets `HIDIdleTime`. Without that reset, the poll after a wake would see `Display == On` and a high idle and turn the screen off again. Idle falling below the threshold sets the belief back to `On` with no action. A notification sets `WakeOwed` and emits `DeclareActivity` even when idle is unknown. `DeclareActivity` succeeding clears `WakeOwed` and sets the belief to `On`. A notification in the same poll as a high idle reading wins, so `TurnDisplayOff` is not emitted. An unknown idle reading with nothing owed leaves the display belief unchanged.

macOS commands live on `macos.System`. The daemon and the CLI call that type through small interfaces. Nothing else runs the commands. The facts below were measured on the owner's Mac, macOS 26, Apple Silicon.

Power comes from the first line of `pmset -g ps`. That line is `Now drawing from 'AC Power'` or `Now drawing from 'Battery Power'`.

Idle comes from the `HIDIdleTime` line of `ioreg -c IOHIDSystem`. The value is nanoseconds.

The sleep assertion is a child process, `caffeinate -s -w <daemon pid>`, started on AC and killed on battery or exit. `pmset -g assertions` then lists `PreventSystemSleep` under the caffeinate pid.

Display off is `pmset displaysleepnow`. The display power state is `ioreg -r -d 1 -c AppleCLCD2`. `"CurrentPowerState"=1` means on and `0` means off. `IODisplayWrangler` is not present on this machine.

Wake is `caffeinate -u -t 1`. The `wake` command is that call. A remote session cannot move the mouse, so the verification skill treats `arch-caffeinate wake` as simulated user activity.

Screen lock is `sysadminctl -screenLock status`, which writes to stderr. The parser reads the phrase `screenLock delay is immediate`, `screenLock delay is off`, or `screenLock delay is <seconds>`. One measured line is `screenLock delay is immediate`. A usage line that merely contains the word `off` is `unknown`.

Notifications are one JSON object per line from `/usr/bin/log stream --style ndjson --predicate 'process == "usernoted" AND subsystem == "com.apple.unc" AND eventMessage BEGINSWITH "Delivering "'`. A delivered notification was measured with `osascript -e 'display notification ...'`.

Tests on Linux inject a clock and a fake `Host`. They compare states and action lists to literal values. `install` takes `HOME` and `ARCH_CAFFEINATE_LAUNCHCTL` so a test can write into a temp directory and record `launchctl` arguments. Running `install` again replaces the plist and bootstraps the same label. The end state matches the latest flags.
