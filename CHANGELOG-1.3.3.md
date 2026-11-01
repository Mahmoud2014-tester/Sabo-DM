# Sabo DM v1.3.3

- Window X hides the WebView2 window while the application and downloads continue.
- System Tray menu provides Open, Pause all, Resume all, and Exit. Exit cancels active transfers and closes the application.
- Windows startup defaults to enabled for new settings and can launch with `--background`.
- Added red-arrow / black-ring application icon and Inno Setup installer with desktop shortcut and startup tasks enabled by default.

This source update requires Windows CI and runtime verification before release.


### Build correction
- Removed an unused System Tray Settings menu declaration that caused Go compilation to fail (`declared and not used: settings`). Settings remain accessible from the application dashboard.
