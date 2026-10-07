# Design

`Step` in `internal/core` is a pure function. It takes a config, the current state, and one observation, and it returns the next state plus a list of actions. `Advance` runs those actions and keeps an effect only when the action succeeds. The observation carries the power source, the HID idle duration when it is known, whether the host currently holds the sleep assertion, and whether a notification was delivered since the last poll. The power source is `AC`, `Battery`, or `Unknown`. The state records two facts. The sleep assertion is held or not. The display is `On` or `Off`.

The assertion is held exactly while power is `AC`. `Unknown` releases it. The host's sleep child is what the next observation reads, so a `caffeinate` process that has already exited is acquired again. The display action `TurnDisplayOff` is emitted once, when idle reaches the threshold and the display is still `On`. Further polls that stay idle do not emit it again. Idle falling below the threshold sets the display back to `On` with no action. A notification emits `DeclareActivity`. A notification in the same poll as a high idle reading wins, so the display stays `On`. An unknown idle reading leaves the display unchanged.

macOS commands live on `macos.System`. The daemon and the CLI call that type through small interfaces. Nothing else runs the commands. The facts below were measured on the owner's Mac, macOS 26, Apple Silicon.

Power comes from the first line of `pmset -g ps`. That line is `Now drawing from 'AC Power'` or `Now drawing from 'Battery Power'`.

Idle comes from the `HIDIdleTime` line of `ioreg -c IOHIDSystem`. The value is nanoseconds.

The sleep assertion is a child process, `caffeinate -s -w <daemon pid>`, started on AC and killed on battery or exit. `pmset -g assertions` then lists `PreventSystemSleep` under the caffeinate pid.

Display off is `pmset displaysleepnow`. The display power state is `ioreg -r -d 1 -c AppleCLCD2`. `"CurrentPowerState"=1` means on and `0` means off. `IODisplayWrangler` is not present on this machine.

Wake is `caffeinate -u -t 1`. The `wake` command is that call. A remote session cannot move the mouse, so the verification skill treats `arch-caffeinate wake` as simulated user activity.

Screen lock is `sysadminctl -screenLock status`, which writes to stderr. The parser reads the phrase `screenLock delay is immediate`, `screenLock delay is off`, or `screenLock delay is <seconds>`. One measured line is `screenLock delay is immediate`. A usage line that merely contains the word `off` is `unknown`.

Notifications are one JSON object per line from `/usr/bin/log stream --style ndjson --predicate 'process == "usernoted" AND subsystem == "com.apple.unc" AND eventMessage BEGINSWITH "Delivering "'`. A delivered notification was measured with `osascript -e 'display notification ...'`.

Tests on Linux inject a clock and a fake `Host`. They compare states and action lists to literal values. `install` takes `HOME` and `ARCH_CAFFEINATE_LAUNCHCTL` so a test can write into a temp directory and record `launchctl` arguments. Running `install` again replaces the plist and bootstraps the same label. The end state matches the latest flags.
