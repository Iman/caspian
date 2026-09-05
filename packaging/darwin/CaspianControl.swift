// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
import AppKit

// All subprocess I/O runs away from the AppKit thread. Network checks use
// URLSession with a deadline. The UI never waits for a process or a socket.
final class Control: NSObject, NSApplicationDelegate, NSWindowDelegate {
    private var window: NSWindow!
    private let state = NSTextField(labelWithString: "Checking local panel…")
    private let detail = NSTextField(wrappingLabelWithString: "")
    private let output = NSTextView()
    private let progress = NSProgressIndicator()
    private var buttons: [NSButton] = []
    private var busy = false
    private var checking = false
    private var timer: Timer?
    private var password: String?
    private var statusItem: NSStatusItem!
    private let menuStatus = NSMenuItem(title: "Checking local panel…", action: nil, keyEquivalent: "")
    private var serviceMenuItems: [NSMenuItem] = []
    private let copy = NSButton(title: "Copy panel password", target: nil, action: nil)
    private let panelURL = URL(string: "http://127.0.0.1:8088/")!

    func applicationDidFinishLaunching(_ notification: Notification) {
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 720, height: 670),
            styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        // Closing a menu-bar app's panel must hide it, not release the AppKit
        // object while this Swift property still points to its old address.
        window.isReleasedWhenClosed = false
        window.delegate = self
        window.title = "Caspian Control"
        window.minSize = NSSize(width: 680, height: 640)
        let stack = NSStackView()
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 14
        stack.translatesAutoresizingMaskIntoConstraints = false
        window.contentView!.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: window.contentView!.leadingAnchor, constant: 24),
            stack.trailingAnchor.constraint(equalTo: window.contentView!.trailingAnchor, constant: -24),
            stack.topAnchor.constraint(equalTo: window.contentView!.topAnchor, constant: 24),
            stack.bottomAnchor.constraint(equalTo: window.contentView!.bottomAnchor, constant: -24)
        ])
        let logo = NSImageView()
        if let url = Bundle.main.url(forResource: "Caspian", withExtension: "icns") { logo.image = NSImage(contentsOf: url) }
        logo.setAccessibilityLabel("Caspian shield and Wi-Fi signal")
        logo.widthAnchor.constraint(equalToConstant: 56).isActive = true
        logo.heightAnchor.constraint(equalToConstant: 56).isActive = true
        let title = NSTextField(labelWithString: "CASPIAN CONTROL")
        title.font = .boldSystemFont(ofSize: 24)
        let header = NSStackView(views: [logo, title]); header.spacing = 16
        stack.addArrangedSubview(header)
        state.font = .boldSystemFont(ofSize: 18)
        stack.addArrangedSubview(state)
        detail.stringValue = "The local panel is separate from the hotspot. Keep this app open so you can recover access even when Wi-Fi is off."
        stack.addArrangedSubview(detail)
        let actions: [(String, String, Int)] = [
            ("Open panel", "Configure your proxy, change passwords, and turn hotspot or client internet on/off.", 0),
            ("Test local panel", "Checks the local web service only; phone internet and proxy egress need separate tests.", 1),
            ("Install / Update", "Install bundled services. First installation displays a new panel password below.", 2),
            ("Reset password", "Forgotten panel password? Authorize on this Mac; a new password appears below.", 3),
            ("Start services", "Restore the background services. Then use Open panel to switch on the hotspot.", 4),
            ("Stop services", "Disconnects clients and closes the web panel. This control window stays available.", 5),
            ("Restart services", "Recover unresponsive services. Clients disconnect; switch the hotspot on afterward.", 6)
        ]
        for (name, explanation, tag) in actions {
            let button = NSButton(title: name, target: self, action: #selector(action(_:)))
            button.tag = tag
            button.bezelStyle = .rounded
            button.toolTip = explanation
            button.widthAnchor.constraint(equalToConstant: 150).isActive = true
            let label = NSTextField(wrappingLabelWithString: explanation)
            label.font = .systemFont(ofSize: 12)
            let row = NSStackView(views: [button, label]); row.spacing = 14
            stack.addArrangedSubview(row)
            row.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            buttons.append(button)
        }
        progress.style = .spinning
        progress.isDisplayedWhenStopped = false
        stack.addArrangedSubview(progress)
        let scroll = NSScrollView()
        scroll.hasVerticalScroller = true
        scroll.borderType = .bezelBorder
        output.isEditable = false
        output.isSelectable = true
        output.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        output.autoresizingMask = [.width]
        output.textContainer?.widthTracksTextView = true
        scroll.documentView = output
        stack.addArrangedSubview(scroll)
        scroll.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
        scroll.heightAnchor.constraint(greaterThanOrEqualToConstant: 90).isActive = true
        copy.target = self; copy.action = #selector(copyPassword); copy.isEnabled = false
        stack.addArrangedSubview(copy)
        let version = Bundle.main.object(forInfoDictionaryKey: "CaspianVersion") as? String ?? "dev"
        stack.addArrangedSubview(NSTextField(labelWithString: "Caspian \(version) · Iman Samizadeh · github.com/Iman/caspian"))
        window.center(); window.makeKeyAndOrderFront(nil)
        configureMenuBar()
        NSApp.activate(ignoringOtherApps: true)
        check()
        timer = Timer.scheduledTimer(withTimeInterval: 5, repeats: true) { [weak self] _ in self?.check() }
        if CommandLine.arguments.count == 3 && CommandLine.arguments[1] == "--screenshot" {
            DispatchQueue.main.asyncAfter(deadline: .now() + 1) {
                // Capture the actual composited window: cacheDisplay misses
                // modern AppKit button layers and can produce blank labels.
                let capture = Process()
                capture.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
                capture.arguments = ["-x", "-o", "-l", String(self.window.windowNumber), CommandLine.arguments[2]]
                capture.terminationHandler = { _ in DispatchQueue.main.async { NSApp.terminate(nil) } }
                do { try capture.run() } catch { NSApp.terminate(nil) }
            }
        }
    }

    // Closing the window does not remove the recovery controls or stop services.
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }

    func windowShouldClose(_ sender: NSWindow) -> Bool {
        sender.orderOut(nil)
        return false
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        showWindow()
        return true
    }

    private func configureMenuBar() {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        let image = Bundle.main.url(forResource: "Caspian", withExtension: "icns").flatMap { NSImage(contentsOf: $0) }
            ?? NSImage(systemSymbolName: "lock.shield", accessibilityDescription: "Caspian")
        image?.size = NSSize(width: 18, height: 18)
        statusItem.button?.image = image
        statusItem.button?.toolTip = "Caspian — checking local panel"
        let menu = NSMenu()
        menu.autoenablesItems = false
        menuStatus.isEnabled = false
        menu.addItem(menuStatus)
        menu.addItem(.separator())
        let show = NSMenuItem(title: "Open Caspian Control", action: #selector(showWindow), keyEquivalent: "")
        show.target = self; menu.addItem(show)
        for (name, tag) in [("Open panel", 0), ("Test local panel", 1), ("Start services", 4), ("Stop services…", 5), ("Restart services…", 6)] {
            let item = NSMenuItem(title: name, action: #selector(menuAction(_:)), keyEquivalent: "")
            item.target = self; item.tag = tag; menu.addItem(item)
            if tag > 1 { serviceMenuItems.append(item) }
        }
        menu.addItem(.separator())
        let quit = NSMenuItem(title: "Quit Control (services keep running)", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        quit.target = NSApp; menu.addItem(quit)
        statusItem.menu = menu
    }

    @objc private func showWindow() {
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    @objc private func menuAction(_ sender: NSMenuItem) {
        guard let button = buttons.first(where: { $0.tag == sender.tag }) else { return }
        if sender.tag > 1 { showWindow() }
        action(button)
    }

    @objc private func copyPassword() {
        guard let password else { return }
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(password, forType: .string)
    }

    @objc private func action(_ sender: NSButton) {
        if sender.tag == 0 { NSWorkspace.shared.open(panelURL); return }
        if sender.tag == 1 { check(); return }
        guard !busy else { return }
        if sender.tag == 5 || sender.tag == 6 {
            let alert = NSAlert()
            alert.messageText = sender.title
            alert.informativeText = "This disconnects hotspot clients and closes their panel sessions. This Mac control window remains available."
            alert.addButton(withTitle: "Continue"); alert.addButton(withTitle: "Cancel")
            alert.beginSheetModal(for: window) { [weak self] result in
                if result == .alertFirstButtonReturn { self?.perform(sender.tag) }
            }
        } else { perform(sender.tag) }
    }

    private func check() {
        guard !checking && !busy else { return }
        checking = true
        var request = URLRequest(url: panelURL, cachePolicy: .reloadIgnoringLocalCacheData, timeoutInterval: 5)
        request.httpMethod = "HEAD"
        URLSession.shared.dataTask(with: request) { [weak self] _, response, error in
            DispatchQueue.main.async {
                guard let self else { return }
                self.checking = false
                guard !self.busy else { return }
                let code = (response as? HTTPURLResponse)?.statusCode ?? 0
                let ready = error == nil && (200..<400).contains(code)
                self.state.stringValue = ready ? "Local panel is reachable" : "Local panel is not reachable"
                self.menuStatus.title = self.state.stringValue
                self.state.textColor = ready ? .systemGreen : .systemOrange
                self.statusItem.button?.toolTip = ready ? "Caspian — local panel reachable; tunnel status is in the panel" : "Caspian — local panel unavailable"
                self.detail.stringValue = ready
                    ? "Open panel to see tunnel and hotspot status. A reachable panel does not prove phone internet works."
                    : "Use Start services if installed, or Install / Update first. The panel always opens at 127.0.0.1:8088 on this Mac."
            }
        }.resume()
    }

    private func perform(_ action: Int) {
        guard !busy, let resources = Bundle.main.resourceURL else { return }
        func quote(_ text: String) -> String { "'" + text.replacingOccurrences(of: "'", with: "'\\''") + "'" }
        let command: String
        switch action {
        case 2:
            command = "CASPIAN_LOCAL_BINARY=" + quote(resources.appendingPathComponent("caspian").path)
                + " /bin/bash " + quote(resources.appendingPathComponent("install-darwin.sh").path)
        case 3: command = "/bin/bash " + quote(resources.appendingPathComponent("reset-password.sh").path)
        case 4, 5, 6:
            let verb = action == 4 ? "start" : action == 5 ? "stop" : "restart"
            command = "/bin/bash " + quote(resources.appendingPathComponent("service-action.sh").path) + " " + verb
        default: return
        }
        busy = true
        buttons.filter { $0.tag > 1 }.forEach { $0.isEnabled = false }
        serviceMenuItems.forEach { $0.isEnabled = false }
        progress.startAnimation(nil)
        state.stringValue = "Waiting for macOS authorization / applying changes…"
        menuStatus.title = "Applying changes…"
        let process = Process()
        let pipe = Pipe()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/osascript")
        process.arguments = ["-e", "on run argv\nreturn do shell script (item 1 of argv) with administrator privileges\nend run", command]
        process.standardOutput = pipe
        process.standardError = pipe
        // This watchdog reports uncertainty instead of starting a second action.
        // Killing osascript cannot guarantee cancellation of its privileged child.
        let watchdog = DispatchWorkItem { [weak self] in
            guard let self, self.busy else { return }
            self.state.stringValue = "Still waiting — changes are not yet confirmed"
            self.detail.stringValue = "Check the macOS authorization dialog. Do not retry while this operation is active. Open panel and window navigation remain available."
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 90, execute: watchdog)
        DispatchQueue.global(qos: .userInitiated).async {
            var result: String
            var success = false
            do {
                try process.run()
                let data = pipe.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                result = String(decoding: data, as: UTF8.self)
                success = process.terminationStatus == 0
            } catch { result = error.localizedDescription }
            DispatchQueue.main.async {
                watchdog.cancel()
                self.busy = false
                self.progress.stopAnimation(nil)
                self.buttons.forEach { $0.isEnabled = true }
                self.serviceMenuItems.forEach { $0.isEnabled = true }
                self.output.string = result
                self.state.stringValue = success ? "Action completed" : "Action failed or authorization cancelled"
                // Do not log or copy the entire installation output. The copy
                // button copies only the newly issued panel credential.
                for line in result.components(separatedBy: .newlines) {
                    for prefix in ["first-run panel password: ", "New Caspian panel password: "] where line.hasPrefix(prefix) {
                        self.password = String(line.dropFirst(prefix.count))
                        self.copy.isEnabled = true
                    }
                }
                self.check()
            }
        }
    }
}

let application = NSApplication.shared
let delegate = Control()
application.setActivationPolicy(.regular)
application.delegate = delegate
let menu = NSMenu()
let appMenu = NSMenu()
appMenu.addItem(withTitle: "Quit Caspian Control", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
let item = NSMenuItem(); item.submenu = appMenu; menu.addItem(item)
application.mainMenu = menu
// NSApplication.delegate is weak. In this hand-written entry point there is
// no app-delegate property generated by an Xcode template to own Control, so
// Swift may release the local after its last apparent use while AppKit keeps
// the menu-item targets as non-owning references. The result is a delayed
// crash when a status-menu action reaches that stale target. Keep the delegate,
// and therefore its window and status item, alive for the entire event loop.
withExtendedLifetime(delegate) {
    application.run()
}
