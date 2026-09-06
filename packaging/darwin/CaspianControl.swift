// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
import AppKit
import Darwin

// AppKit's standard rounded button truncates a two-line title even when its
// cell is configured to wrap. Keep the native NSButton interaction and
// accessibility behaviour, but draw two explicit, high-contrast language
// lines so neither an English nor Persian reader has to guess the action.
final class BilingualButton: NSButton {
    var englishTitle = "" { didSet { title = englishTitle; needsDisplay = true } }
    var persianTitle = "" { didSet { needsDisplay = true } }
    var isPrimaryAction = false { didSet { needsDisplay = true } }

    override func highlight(_ flag: Bool) {
        super.highlight(flag)
        // AppKit normally invalidates only the button cell's title strip when
        // its pressed state changes. Our custom drawing covers the complete
        // control, so repaint everything to avoid leaving a blue strip behind.
        setNeedsDisplay(bounds)
    }

    override func draw(_ dirtyRect: NSRect) {
        let rect = bounds.insetBy(dx: 1.5, dy: 1.5)
        let path = NSBezierPath(roundedRect: rect, xRadius: 10, yRadius: 10)
        let accented = isPrimaryAction || isHighlighted
        if !isEnabled {
            NSColor.controlBackgroundColor.setFill()
            NSColor.separatorColor.setStroke()
        } else if accented {
            NSColor.controlAccentColor.setFill()
            NSColor.controlAccentColor.setStroke()
        } else {
            NSColor.controlBackgroundColor.setFill()
            NSColor.controlAccentColor.setStroke()
        }
        path.fill()
        path.lineWidth = isEnabled && !accented ? 2 : 1
        path.stroke()

        let color: NSColor = !isEnabled ? .disabledControlTextColor : accented ? .white : .labelColor
        // NSButton's drawing context is flipped: the lower-coordinate rect is
        // visually the first line. Keep English above Persian as promised.
        drawLine(englishTitle, in: NSRect(x: 10, y: bounds.midY - 24, width: bounds.width - 20, height: 24),
            direction: .leftToRight, color: color)
        drawLine(persianTitle, in: NSRect(x: 10, y: bounds.midY + 2, width: bounds.width - 20, height: 24),
            direction: .rightToLeft, color: color)
    }

    private func drawLine(
        _ text: String,
        in rect: NSRect,
        direction: NSWritingDirection,
        color: NSColor
    ) {
        let style = NSMutableParagraphStyle()
        style.alignment = .center
        style.baseWritingDirection = direction
        style.lineBreakMode = .byTruncatingTail
        (text as NSString).draw(
            in: rect,
            withAttributes: [
                .font: NSFont.systemFont(ofSize: 17, weight: .semibold),
                .foregroundColor: color,
                .paragraphStyle: style
            ])
    }
}

// All subprocess I/O runs away from the AppKit thread. Network checks use
// URLSession with a deadline. The UI never waits for a process or a socket.
final class Control: NSObject, NSApplicationDelegate, NSWindowDelegate {
    private enum Layout {
        static let windowWidth: CGFloat = 920
        static let windowHeight: CGFloat = 700
        static let outerMargin: CGFloat = 32
        static let sectionGap: CGFloat = 24
        static let rowGap: CGFloat = 16
        static let cardPadding: CGFloat = 20
        static let buttonWidth: CGFloat = 248
        static let buttonHeight: CGFloat = 72
    }

    private enum InstallationState: Equatable {
        case checking
        case notInstalled
        case updateRequired
        case current
        case bundleUnavailable
    }

    private var window: NSWindow!
    private var contentScroll: NSScrollView!
    private let state = NSTextField(wrappingLabelWithString: "")
    private let detail = NSTextField(wrappingLabelWithString: "")
    private let output = NSTextView()
    private var outputScroll: NSScrollView!
    private let progress = NSProgressIndicator()
    private var buttons: [NSButton] = []
    private var actionRows: [Int: NSView] = [:]
    private var setupButton: NSButton!
    private var openPanelButton: NSButton!
    private var recoveryStack: NSStackView!
    private var recoveryToggle: NSButton!
    private var recoveryToggleRow: NSStackView!
    private var busy = false
    private var checking = false
    private var quitting = false
    private var installationState = InstallationState.checking
    private var automaticSetupAttempted = false
    private var timer: Timer?
    private var password: String?
    private var statusItem: NSStatusItem!
    private let menuStatus = NSMenuItem(title: "Checking Caspian… / در حال بررسی کاسپین…", action: nil, keyEquivalent: "")
    private var serviceMenuItems: [NSMenuItem] = []
    private var panelMenuItems: [NSMenuItem] = []
    private let copy = BilingualButton(frame: .zero)
    private let panelURL = URL(string: "http://127.0.0.1:8088/")!
    private let githubURL = URL(string: "https://github.com/Iman/caspian")!
    private let installedBinary = "/usr/local/bin/caspian"

    private var bundledVersion: String {
        Bundle.main.object(forInfoDictionaryKey: "CaspianVersion") as? String ?? "dev"
    }

    private var releaseTag: String? {
        guard let range = bundledVersion.range(
            of: #"^v[0-9]+\.[0-9]+\.[0-9]+"#,
            options: .regularExpression) else { return nil }
        return String(bundledVersion[range])
    }

    private var releaseURL: URL {
        let releases = githubURL.appendingPathComponent("releases")
        guard let releaseTag else { return releases.appendingPathComponent("latest") }
        return releases.appendingPathComponent("tag").appendingPathComponent(releaseTag)
    }

    private var screenshotPath: String? {
        if let value = getenv("CASPIAN_SCREENSHOT_PATH") {
            let path = String(cString: value)
            if !path.isEmpty { return path }
        }
        guard let flag = CommandLine.arguments.firstIndex(of: "--screenshot") else { return nil }
        let value = CommandLine.arguments.index(after: flag)
        guard value < CommandLine.arguments.endIndex else { return nil }
        return CommandLine.arguments[value]
    }

    private var screenshotMode: Bool { screenshotPath != nil }

    private func bilingual(_ english: String, _ persian: String) -> String {
        english + "\n" + persian
    }

    private func setBilingualText(
        _ field: NSTextField,
        english: String,
        persian: String,
        size: CGFloat,
        weight: NSFont.Weight = .regular,
        color: NSColor = .labelColor
    ) {
        let englishStyle = NSMutableParagraphStyle()
        englishStyle.alignment = .left
        englishStyle.baseWritingDirection = .leftToRight
        englishStyle.paragraphSpacing = 4
        let persianStyle = NSMutableParagraphStyle()
        // Keep the Persian translation directly beneath the English starting
        // edge. Its base direction remains RTL, preserving correct shaping and
        // reading order without splitting the panel into competing columns.
        persianStyle.alignment = .left
        persianStyle.baseWritingDirection = .rightToLeft
        let font = NSFont.systemFont(ofSize: size, weight: weight)
        let text = NSMutableAttributedString(
            string: english + "\n",
            attributes: [.font: font, .foregroundColor: color, .paragraphStyle: englishStyle])
        text.append(NSAttributedString(
            string: persian,
            attributes: [.font: font, .foregroundColor: color, .paragraphStyle: persianStyle]))
        field.attributedStringValue = text
        field.maximumNumberOfLines = 0
        field.lineBreakMode = .byWordWrapping
        field.cell?.wraps = true
        field.cell?.usesSingleLineMode = false
        field.setContentHuggingPriority(.required, for: .vertical)
        field.setContentCompressionResistancePriority(.required, for: .vertical)
    }

    private func setState(_ english: String, _ persian: String, color: NSColor = .labelColor) {
        setBilingualText(state, english: english, persian: persian, size: 22, weight: .bold, color: color)
    }

    private func setDetail(_ english: String, _ persian: String) {
        setBilingualText(detail, english: english, persian: persian, size: 17)
    }

    private func setButtonTitle(_ button: NSButton, _ english: String, _ persian: String) {
        if let bilingualButton = button as? BilingualButton {
            bilingualButton.englishTitle = english
            bilingualButton.persianTitle = persian
        } else {
            button.title = bilingual(english, persian)
        }
        button.setAccessibilityLabel(english + ". " + persian)
    }

    private func makeCard(containing content: NSView) -> NSBox {
        let card = NSBox()
        card.boxType = .custom
        card.titlePosition = .noTitle
        card.borderColor = .separatorColor
        card.borderWidth = 1
        card.cornerRadius = 12
        card.fillColor = .controlBackgroundColor
        card.contentViewMargins = .zero
        guard let host = card.contentView else { return card }
        content.translatesAutoresizingMaskIntoConstraints = false
        host.addSubview(content)
        NSLayoutConstraint.activate([
            content.leadingAnchor.constraint(equalTo: host.leadingAnchor, constant: Layout.cardPadding),
            content.trailingAnchor.constraint(equalTo: host.trailingAnchor, constant: -Layout.cardPadding),
            content.topAnchor.constraint(equalTo: host.topAnchor, constant: Layout.cardPadding),
            content.bottomAnchor.constraint(equalTo: host.bottomAnchor, constant: -Layout.cardPadding)
        ])
        return card
    }

    private func makeTextLink(_ title: String, direction: NSWritingDirection) -> NSButton {
        let button = NSButton(title: "", target: self, action: #selector(openGitHubProject))
        button.isBordered = false
        button.focusRingType = .none
        button.controlSize = .large
        button.alignment = .left
        let style = NSMutableParagraphStyle()
        style.alignment = .left
        style.baseWritingDirection = direction
        button.attributedTitle = NSAttributedString(
            string: title,
            attributes: [
                .font: NSFont.systemFont(ofSize: 16, weight: .semibold),
                .foregroundColor: NSColor.linkColor,
                .underlineStyle: NSUnderlineStyle.single.rawValue,
                .paragraphStyle: style
            ])
        button.setAccessibilityLabel(title)
        button.toolTip = githubURL.absoluteString
        button.setContentHuggingPriority(.required, for: .horizontal)
        button.setContentCompressionResistancePriority(.required, for: .vertical)
        return button
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        window = NSWindow(contentRect: NSRect(
                x: 0, y: 0, width: Layout.windowWidth, height: Layout.windowHeight),
            styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        // Closing a menu-bar app's panel must hide it, not release the AppKit
        // object while this Swift property still points to its old address.
        window.isReleasedWhenClosed = false
        window.delegate = self
        window.title = "Caspian Control — کنترل کاسپین"
        window.minSize = NSSize(width: 820, height: 620)

        contentScroll = NSScrollView()
        contentScroll.translatesAutoresizingMaskIntoConstraints = false
        contentScroll.drawsBackground = false
        contentScroll.hasVerticalScroller = true
        contentScroll.hasHorizontalScroller = false
        contentScroll.autohidesScrollers = true
        contentScroll.scrollerStyle = .overlay
        contentScroll.allowsMagnification = false
        contentScroll.automaticallyAdjustsContentInsets = false
        window.contentView!.addSubview(contentScroll)
        NSLayoutConstraint.activate([
            contentScroll.leadingAnchor.constraint(equalTo: window.contentView!.leadingAnchor),
            contentScroll.trailingAnchor.constraint(equalTo: window.contentView!.trailingAnchor),
            contentScroll.topAnchor.constraint(equalTo: window.contentView!.topAnchor),
            contentScroll.bottomAnchor.constraint(equalTo: window.contentView!.bottomAnchor)
        ])

        let stack = NSStackView()
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = Layout.sectionGap
        stack.edgeInsets = NSEdgeInsets(
            top: Layout.outerMargin,
            left: Layout.outerMargin,
            bottom: Layout.outerMargin,
            right: Layout.outerMargin)
        stack.translatesAutoresizingMaskIntoConstraints = false
        contentScroll.documentView = stack
        NSLayoutConstraint.activate([
            stack.widthAnchor.constraint(equalTo: contentScroll.contentView.widthAnchor),
            stack.heightAnchor.constraint(greaterThanOrEqualTo: contentScroll.contentView.heightAnchor)
        ])

        func addFullWidth(_ view: NSView) {
            stack.addArrangedSubview(view)
            view.widthAnchor.constraint(
                equalTo: stack.widthAnchor,
                constant: -(Layout.outerMargin * 2)).isActive = true
        }

        let logo = NSImageView()
        if let url = Bundle.main.url(forResource: "Caspian", withExtension: "icns") { logo.image = NSImage(contentsOf: url) }
        logo.setAccessibilityLabel("Caspian shield and Wi-Fi signal")
        logo.widthAnchor.constraint(equalToConstant: 56).isActive = true
        logo.heightAnchor.constraint(equalToConstant: 56).isActive = true
        let title = NSTextField(wrappingLabelWithString: "")
        setBilingualText(title, english: "CASPIAN CONTROL", persian: "کنترل کاسپین", size: 26, weight: .bold)
        title.setContentHuggingPriority(.defaultLow, for: .horizontal)
        let header = NSStackView(views: [logo, title])
        header.spacing = Layout.rowGap
        header.alignment = .centerY
        addFullWidth(header)

        let statusStack = NSStackView(views: [state, detail])
        statusStack.orientation = .vertical
        statusStack.alignment = .leading
        statusStack.spacing = 12
        state.widthAnchor.constraint(equalTo: statusStack.widthAnchor).isActive = true
        detail.widthAnchor.constraint(equalTo: statusStack.widthAnchor).isActive = true
        setState("Checking Caspian…", "در حال بررسی کاسپین…")
        setDetail("Checking whether this version is installed.", "در حال بررسی نصب بودن این نسخه.")
        addFullWidth(makeCard(containing: statusStack))

        let primaryActions = NSStackView()
        primaryActions.orientation = .vertical
        primaryActions.alignment = .leading
        primaryActions.spacing = Layout.rowGap

        func addActionRow(
            to container: NSStackView,
            _ englishName: String,
            _ persianName: String,
            _ englishExplanation: String,
            _ persianExplanation: String,
            _ tag: Int,
            primary: Bool = false
        ) {
            let button = BilingualButton(frame: .zero)
            button.target = self
            button.action = #selector(action(_:))
            setButtonTitle(button, englishName, persianName)
            button.tag = tag
            button.isBordered = false
            button.isPrimaryAction = primary
            button.controlSize = .large
            button.font = .systemFont(ofSize: primary ? 18 : 17, weight: .semibold)
            button.cell?.wraps = true
            button.cell?.usesSingleLineMode = false
            button.toolTip = bilingual(englishExplanation, persianExplanation)
            button.widthAnchor.constraint(equalToConstant: Layout.buttonWidth).isActive = true
            button.heightAnchor.constraint(equalToConstant: Layout.buttonHeight).isActive = true
            button.setContentCompressionResistancePriority(.required, for: .vertical)
            let label = NSTextField(wrappingLabelWithString: "")
            setBilingualText(
                label,
                english: englishExplanation,
                persian: persianExplanation,
                size: primary ? 17 : 16)
            label.setContentHuggingPriority(.defaultLow, for: .horizontal)
            label.setContentCompressionResistancePriority(.required, for: .vertical)
            let row = NSStackView(views: [button, label])
            row.spacing = Layout.rowGap
            row.alignment = .centerY
            container.addArrangedSubview(row)
            row.widthAnchor.constraint(equalTo: container.widthAnchor).isActive = true
            buttons.append(button)
            actionRows[tag] = row
            if tag == 2 { setupButton = button }
            if tag == 0 { openPanelButton = button }
        }

        addActionRow(to: primaryActions,
            "Set up Caspian",
            "راه‌اندازی کاسپین",
            "Installs the secure background services. macOS asks for an administrator password only when setup or an update is needed.",
            "سرویس‌های امن پس‌زمینه را نصب می‌کند. macOS فقط هنگام راه‌اندازی یا به‌روزرسانی رمز مدیر را می‌خواهد.",
            2,
            primary: true)
        addActionRow(to: primaryActions,
            "Open panel",
            "باز کردن پنل",
            "Choose your proxy and turn Caspian protection on or off.",
            "پروکسی خود را انتخاب کنید و محافظت کاسپین را روشن یا خاموش کنید.",
            0,
            primary: true)
        setupButton.isEnabled = false
        openPanelButton.isEnabled = false
        openPanelButton.keyEquivalent = "\r"
        addFullWidth(makeCard(containing: primaryActions))

        recoveryToggle = BilingualButton(frame: .zero)
        recoveryToggle.target = self
        recoveryToggle.action = #selector(toggleRecovery(_:))
        setButtonTitle(recoveryToggle, "Advanced options", "گزینه‌های پیشرفته")
        recoveryToggle.isBordered = false
        recoveryToggle.controlSize = .large
        recoveryToggle.font = .systemFont(ofSize: 17, weight: .semibold)
        recoveryToggle.cell?.wraps = true
        recoveryToggle.cell?.usesSingleLineMode = false
        recoveryToggle.widthAnchor.constraint(equalToConstant: Layout.buttonWidth).isActive = true
        recoveryToggle.heightAnchor.constraint(equalToConstant: Layout.buttonHeight).isActive = true
        recoveryToggle.setContentCompressionResistancePriority(.required, for: .vertical)
        recoveryToggle.toolTip = bilingual(
            "Shows repair, password and service controls.",
            "ابزارهای تعمیر، رمز و کنترل سرویس‌ها را نشان می‌دهد.")
        let recoveryExplanation = NSTextField(wrappingLabelWithString: "")
        setBilingualText(
            recoveryExplanation,
            english: "Repair the installation, reset the panel password, or manage background services.",
            persian: "نصب را تعمیر کنید، رمز پنل را بازنشانی کنید یا سرویس‌های پس‌زمینه را مدیریت کنید.",
            size: 16)
        recoveryExplanation.setContentHuggingPriority(.defaultLow, for: .horizontal)
        recoveryExplanation.setContentCompressionResistancePriority(.required, for: .vertical)
        recoveryToggleRow = NSStackView(views: [recoveryToggle, recoveryExplanation])
        recoveryToggleRow.spacing = Layout.rowGap
        recoveryToggleRow.alignment = .centerY
        recoveryToggleRow.isHidden = true
        addFullWidth(recoveryToggleRow)

        let recoveryRows = NSStackView()
        recoveryRows.orientation = .vertical
        recoveryRows.alignment = .leading
        recoveryRows.spacing = Layout.rowGap
        recoveryStack = NSStackView()
        recoveryStack.orientation = .vertical
        recoveryStack.alignment = .leading
        recoveryStack.spacing = 0
        recoveryStack.isHidden = true
        let recoveryCard = makeCard(containing: recoveryRows)
        recoveryStack.addArrangedSubview(recoveryCard)
        recoveryCard.widthAnchor.constraint(equalTo: recoveryStack.widthAnchor).isActive = true
        addFullWidth(recoveryStack)
        addActionRow(to: recoveryRows,
            "Check again",
            "بررسی دوباره",
            "Rechecks the local panel. This does not prove that internet traffic is using the tunnel.",
            "پنل محلی را دوباره بررسی می‌کند. این بررسی عبور اینترنت از تونل را تأیید نمی‌کند.",
            1)
        addActionRow(to: recoveryRows,
            "Reset password",
            "بازنشانی رمز",
            "Creates a new panel password without deleting your proxy or hotspot settings.",
            "بدون پاک کردن تنظیمات پروکسی یا هات‌اسپات، رمز جدیدی برای پنل می‌سازد.",
            3)
        addActionRow(to: recoveryRows,
            "Start services",
            "راه‌اندازی سرویس‌ها",
            "Restores the background services if they were stopped.",
            "اگر سرویس‌های پس‌زمینه متوقف شده باشند، آن‌ها را دوباره راه‌اندازی می‌کند.",
            4)
        addActionRow(to: recoveryRows,
            "Stop services",
            "توقف سرویس‌ها",
            "Disconnects clients and closes the web panel. This control app stays open.",
            "دستگاه‌ها را قطع و پنل وب را می‌بندد. این برنامهٔ کنترل باز می‌ماند.",
            5)
        addActionRow(to: recoveryRows,
            "Restart services",
            "راه‌اندازی دوبارهٔ سرویس‌ها",
            "Repairs unresponsive background services. Connected clients are briefly disconnected.",
            "سرویس‌های پاسخ‌نداده را تعمیر می‌کند. دستگاه‌های متصل برای مدت کوتاهی قطع می‌شوند.",
            6)

        progress.style = .spinning
        progress.isDisplayedWhenStopped = false
        progress.isHidden = true
        stack.addArrangedSubview(progress)
        outputScroll = NSScrollView()
        outputScroll.hasVerticalScroller = true
        outputScroll.borderType = .lineBorder
        output.isEditable = false
        output.isSelectable = true
        output.font = .monospacedSystemFont(ofSize: 15, weight: .regular)
        output.autoresizingMask = [.width]
        output.textContainer?.widthTracksTextView = true
        outputScroll.documentView = output
        addFullWidth(outputScroll)
        outputScroll.heightAnchor.constraint(equalToConstant: 160).isActive = true
        outputScroll.isHidden = true
        setButtonTitle(copy, "Copy panel password", "کپی رمز پنل")
        copy.target = self; copy.action = #selector(copyPassword); copy.isEnabled = false; copy.isHidden = true
        copy.isBordered = false
        copy.controlSize = .large
        copy.font = .systemFont(ofSize: 17, weight: .semibold)
        copy.cell?.wraps = true
        copy.cell?.usesSingleLineMode = false
        copy.widthAnchor.constraint(equalToConstant: Layout.buttonWidth).isActive = true
        copy.heightAnchor.constraint(equalToConstant: Layout.buttonHeight).isActive = true
        copy.setContentCompressionResistancePriority(.required, for: .vertical)
        stack.addArrangedSubview(copy)
        let exactRelease = releaseTag == bundledVersion
        let versionEnglish: String
        let versionPersian: String
        if exactRelease {
            versionEnglish = "CI release \(bundledVersion)"
            versionPersian = "انتشار CI نسخهٔ \(bundledVersion)"
        } else if let releaseTag {
            versionEnglish = "CI \(releaseTag) · UX preview"
            versionPersian = "پیش‌نمایش رابط بر پایهٔ \(releaseTag)"
        } else {
            versionEnglish = "Test build \(bundledVersion)"
            versionPersian = "نسخهٔ آزمایشی \(bundledVersion)"
        }
        let version = NSTextField(wrappingLabelWithString: "")
        setBilingualText(
            version,
            english: versionEnglish,
            persian: versionPersian,
            size: 15,
            weight: .semibold,
            color: .secondaryLabelColor)

        let githubEnglish = makeTextLink("Open GitHub project", direction: .leftToRight)
        let githubPersian = makeTextLink("باز کردن پروژه در گیت‌هاب", direction: .rightToLeft)
        let github = NSStackView(views: [githubEnglish, githubPersian])
        github.orientation = .vertical
        github.alignment = .leading
        github.spacing = 4

        let owner = NSTextField(wrappingLabelWithString: "")
        setBilingualText(
            owner,
            english: "Iman Samizadeh",
            persian: "ایمان سمیع زاده",
            size: 16,
            weight: .medium,
            color: .secondaryLabelColor)

        let footerSeparator = NSBox()
        footerSeparator.boxType = .separator
        addFullWidth(footerSeparator)
        let footer = NSStackView(views: [version, github, owner])
        footer.spacing = Layout.sectionGap
        footer.alignment = .top
        footer.distribution = .fillEqually
        addFullWidth(footer)
        window.center(); window.makeKeyAndOrderFront(nil)
        configureMenuBar()
        NSApp.activate(ignoringOtherApps: true)
        check(offerAutomaticSetup: !screenshotMode)
        timer = Timer.scheduledTimer(withTimeInterval: 5, repeats: true) { [weak self] _ in self?.check() }
        if let screenshotPath {
            DispatchQueue.main.asyncAfter(deadline: .now() + 1) { [weak self] in
                self?.saveScreenshotAndQuit(at: screenshotPath)
            }
        }
    }

    private func saveScreenshotAndQuit(at path: String) {
        defer { exit(0) }
        guard let view = window.contentView else { return }
        window.displayIfNeeded()
        let bounds = view.bounds
        guard let image = NSBitmapImageRep(
            bitmapDataPlanes: nil,
            pixelsWide: Int(bounds.width),
            pixelsHigh: Int(bounds.height),
            bitsPerSample: 8,
            samplesPerPixel: 4,
            hasAlpha: true,
            isPlanar: false,
            colorSpaceName: .deviceRGB,
            bytesPerRow: 0,
            bitsPerPixel: 0),
            let context = NSGraphicsContext(bitmapImageRep: image) else { return }
        image.size = bounds.size
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = context
        view.displayIgnoringOpacity(bounds, in: context)
        NSGraphicsContext.restoreGraphicsState()
        guard let png = image.representation(using: .png, properties: [:]) else { return }
        try? png.write(to: URL(fileURLWithPath: path), options: .atomic)
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
        let show = NSMenuItem(title: "Open Caspian Control / باز کردن کنترل کاسپین", action: #selector(showWindow), keyEquivalent: "")
        show.target = self; menu.addItem(show)
        for (name, tag) in [
            ("Open panel / باز کردن پنل", 0),
            ("Test local panel / بررسی پنل محلی", 1),
            ("Start services / راه‌اندازی سرویس‌ها", 4),
            ("Stop services… / توقف سرویس‌ها…", 5),
            ("Restart services… / راه‌اندازی دوباره…", 6)
        ] {
            let item = NSMenuItem(title: name, action: #selector(menuAction(_:)), keyEquivalent: "")
            item.target = self; item.tag = tag; menu.addItem(item)
            if tag > 1 { serviceMenuItems.append(item) } else { panelMenuItems.append(item) }
        }
        menu.addItem(.separator())
        let quit = NSMenuItem(title: "Quit Caspian and stop services / خروج و توقف سرویس‌ها", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        quit.target = NSApp; menu.addItem(quit)
        statusItem.menu = menu
    }

    @objc private func showWindow() {
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    @objc private func openGitHubProject() {
        NSWorkspace.shared.open(githubURL)
    }

    @objc private func openReleasePage() {
        NSWorkspace.shared.open(releaseURL)
    }

    @objc private func toggleRecovery(_ sender: NSButton) {
        let showing = recoveryStack.isHidden
        recoveryStack.isHidden = !showing
        if showing {
            setButtonTitle(sender, "Hide advanced options", "بستن گزینه‌های پیشرفته")
        } else {
            setButtonTitle(sender, "Advanced options", "گزینه‌های پیشرفته")
        }
        resizeWindowForContent()
    }

    private func hideRecovery() {
        recoveryStack.isHidden = true
        setButtonTitle(recoveryToggle, "Advanced options", "گزینه‌های پیشرفته")
    }

    private func resizeWindowForContent(animated _: Bool = true) {
        // Variable sections live in the scroll document. Re-lay out in place
        // instead of resizing the window, which made every status refresh feel
        // like the typography and grid were jumping.
        window.contentView?.layoutSubtreeIfNeeded()
        let target: NSView? = !outputScroll.isHidden
            ? outputScroll
            : (!recoveryStack.isHidden ? recoveryStack : nil)
        if let target {
            DispatchQueue.main.async { target.scrollToVisible(target.bounds) }
        }
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
        if sender.tag == 0 {
            guard installationState == .current && openPanelButton.isEnabled else { showWindow(); return }
            NSWorkspace.shared.open(panelURL)
            return
        }
        if sender.tag == 1 { check(); return }
        guard !busy else { return }
        if sender.tag == 5 || sender.tag == 6 {
            let alert = NSAlert()
            alert.messageText = sender.title
            alert.informativeText = bilingual(
                "This disconnects hotspot clients and closes their panel sessions. This Mac control window remains available.",
                "این کار دستگاه‌های هات‌اسپات را قطع و نشست‌های پنل را می‌بندد. پنجرهٔ کنترل روی این مک باز می‌ماند.")
            alert.addButton(withTitle: "Continue / ادامه"); alert.addButton(withTitle: "Cancel / لغو")
            alert.beginSheetModal(for: window) { [weak self] result in
                if result == .alertFirstButtonReturn { self?.perform(sender.tag) }
            }
        } else { perform(sender.tag) }
    }

    private func inspectInstallation() -> InstallationState {
        guard let bundled = Bundle.main.resourceURL?.appendingPathComponent("caspian"),
              FileManager.default.fileExists(atPath: bundled.path) else {
            return .bundleUnavailable
        }
        guard FileManager.default.fileExists(atPath: installedBinary) else {
            return .notInstalled
        }
        return FileManager.default.contentsEqual(atPath: bundled.path, andPath: installedBinary)
            ? .current : .updateRequired
    }

    private func check(offerAutomaticSetup: Bool = false) {
        guard !checking && !busy else { return }
        checking = true
        DispatchQueue.global(qos: .utility).async { [weak self] in
            guard let self else { return }
            let installation = self.inspectInstallation()
            DispatchQueue.main.async {
                guard !self.busy else { self.checking = false; return }
                self.installationState = installation
                self.applyInstallationState(offerAutomaticSetup: offerAutomaticSetup)
            }
        }
    }

    private func applyInstallationState(offerAutomaticSetup: Bool) {
        switch installationState {
        case .checking:
            checking = false
            return
        case .bundleUnavailable:
            checking = false
            setupButton.isEnabled = false
            openPanelButton.isEnabled = false
            setState("This copy of Caspian is incomplete", "این نسخهٔ کاسپین ناقص است", color: .systemRed)
            setDetail(
                "Download Caspian again. The secure background-service binary is missing from this app.",
                "کاسپین را دوباره دانلود کنید. فایل سرویس امن پس‌زمینه در این برنامه وجود ندارد.")
            menuStatus.title = "Caspian app is incomplete / برنامهٔ کاسپین ناقص است"
            hideRecovery()
            recoveryToggleRow.isHidden = true
            resizeWindowForContent()
            serviceMenuItems.forEach { $0.isEnabled = false }
            panelMenuItems.forEach { $0.isEnabled = false }
            return
        case .notInstalled, .updateRequired:
            checking = false
            let update = installationState == .updateRequired
            setButtonTitle(
                setupButton,
                update ? "Update Caspian" : "Set up Caspian",
                update ? "به‌روزرسانی کاسپین" : "راه‌اندازی کاسپین")
            setupButton.isEnabled = true
            setupButton.keyEquivalent = "\r"
            openPanelButton.keyEquivalent = ""
            actionRows[2]?.isHidden = false
            actionRows[0]?.isHidden = true
            openPanelButton.isEnabled = false
            setState(
                update ? "Caspian needs an update" : "Welcome to Caspian",
                update ? "کاسپین به به‌روزرسانی نیاز دارد" : "به کاسپین خوش آمدید")
            if update {
                setDetail(
                    "This app contains a newer background service. Caspian will update it without deleting your settings.",
                    "این برنامه سرویس پس‌زمینهٔ جدیدتری دارد. کاسپین آن را بدون پاک کردن تنظیمات شما به‌روزرسانی می‌کند.")
            } else {
                setDetail(
                    "Caspian installs its background services without extra tools or Terminal. macOS will ask for an administrator password.",
                    "کاسپین سرویس‌های پس‌زمینه را بدون ابزار اضافی یا ترمینال نصب می‌کند. macOS رمز مدیر را درخواست خواهد کرد.")
            }
            menuStatus.title = update
                ? "Update required / به‌روزرسانی لازم است"
                : "Setup required / راه‌اندازی لازم است"
            statusItem.button?.toolTip = "Caspian — " + menuStatus.title.lowercased()
            hideRecovery()
            recoveryToggleRow.isHidden = true
            resizeWindowForContent()
            serviceMenuItems.forEach { $0.isEnabled = false }
            panelMenuItems.forEach { $0.isEnabled = false }
            if offerAutomaticSetup && !automaticSetupAttempted && !screenshotMode {
                automaticSetupAttempted = true
                DispatchQueue.main.async { [weak self] in self?.perform(2) }
            }
            return
        case .current:
            setupButton.isEnabled = false
            setupButton.keyEquivalent = ""
            openPanelButton.keyEquivalent = "\r"
            openPanelButton.isEnabled = false
            actionRows[2]?.isHidden = true
            actionRows[0]?.isHidden = false
            recoveryToggleRow.isHidden = false
            serviceMenuItems.forEach { $0.isEnabled = true }
            panelMenuItems.forEach { $0.isEnabled = true }
            checkPanel(offerAutomaticStart: offerAutomaticSetup)
        }
    }

    private func checkPanel(offerAutomaticStart: Bool = false) {
        var request = URLRequest(url: panelURL, cachePolicy: .reloadIgnoringLocalCacheData, timeoutInterval: 5)
        request.httpMethod = "HEAD"
        URLSession.shared.dataTask(with: request) { [weak self] _, response, error in
            DispatchQueue.main.async {
                guard let self else { return }
                self.checking = false
                guard !self.busy else { return }
                let code = (response as? HTTPURLResponse)?.statusCode ?? 0
                let ready = error == nil && (200..<400).contains(code)
                // Reopening after Quit restores the services once. Periodic
                // checks must not undo a deliberate Stop services action.
                if !ready && offerAutomaticStart && !self.automaticSetupAttempted && !self.screenshotMode {
                    self.automaticSetupAttempted = true
                    self.perform(4)
                    return
                }
                self.openPanelButton.isEnabled = ready
                self.panelMenuItems.first(where: { $0.tag == 0 })?.isEnabled = ready
                self.setState(
                    ready ? "Caspian is ready" : "Caspian needs attention",
                    ready ? "کاسپین آماده است" : "کاسپین نیاز به بررسی دارد",
                    color: ready ? .systemGreen : .systemOrange)
                self.menuStatus.title = ready ? "Caspian is ready / کاسپین آماده است" : "Caspian needs attention / کاسپین نیاز به بررسی دارد"
                self.statusItem.button?.toolTip = ready ? "Caspian — local panel reachable; tunnel status is in the panel" : "Caspian — local panel unavailable"
                if ready && self.password != nil {
                    self.setDetail(
                        "Save the new panel password below, then open the panel. The panel shows whether tunnel protection is actually on.",
                        "رمز جدید پنل را که پایین آمده ذخیره کنید و سپس پنل را باز کنید. پنل روشن بودن واقعی محافظت تونل را نشان می‌دهد.")
                } else {
                    if ready {
                        self.setDetail(
                            "Open the panel to choose your proxy and turn protection on. A reachable panel alone does not mean the tunnel is on.",
                            "پنل را باز کنید، پروکسی را انتخاب کنید و محافظت را روشن کنید. در دسترس بودن پنل به‌تنهایی به معنی روشن بودن تونل نیست.")
                    } else {
                        self.setDetail(
                            "The correct version is installed, but its panel is not responding. Open Advanced options and choose Start services.",
                            "نسخهٔ درست نصب شده، اما پنل پاسخ نمی‌دهد. گزینه‌های پیشرفته را باز کنید و راه‌اندازی سرویس‌ها را بزنید.")
                    }
                }
            }
        }.resume()
    }

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        if CommandLine.arguments.contains("--screenshot") { return .terminateNow }
        if busy { showWindow(); return .terminateCancel }
        let installed = FileManager.default.fileExists(atPath:
            "/Library/LaunchDaemons/org.caspianbyoc.caspian.plist")
            || FileManager.default.fileExists(atPath:
                "/Library/LaunchDaemons/org.caspianbyoc.caspian-panel.plist")
        if !installed { return .terminateNow }
        quitting = true
        showWindow()
        perform(5)
        return .terminateLater
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
        progress.isHidden = false
        progress.startAnimation(nil)
        setState(
            action == 2 ? "Setting up Caspian…" : "Applying changes…",
            action == 2 ? "در حال راه‌اندازی کاسپین…" : "در حال اعمال تغییرات…")
        if action == 2 {
            setDetail(
                "Use the macOS administrator prompt to approve Caspian's secure background services.",
                "در پنجرهٔ macOS رمز مدیر را وارد کنید تا سرویس‌های امن پس‌زمینهٔ کاسپین تأیید شوند.")
        } else {
            setDetail("Waiting for macOS authorization.", "در انتظار تأیید macOS.")
        }
        menuStatus.title = "Applying changes… / در حال اعمال تغییرات…"
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
            self.setState(
                "Still waiting — changes are not yet confirmed",
                "هنوز منتظریم — تغییرات هنوز تأیید نشده‌اند",
                color: .systemOrange)
            self.setDetail(
                "Check the macOS authorization dialog. Do not retry while this operation is active.",
                "پنجرهٔ تأیید macOS را بررسی کنید. تا پایان این عملیات دوباره تلاش نکنید.")
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
                self.progress.isHidden = true
                self.buttons.forEach { $0.isEnabled = true }
                self.serviceMenuItems.forEach { $0.isEnabled = true }
                if self.quitting {
                    self.quitting = false
                    if success { self.timer?.invalidate() }
                    NSApp.reply(toApplicationShouldTerminate: success)
                    if success { return }
                }
                self.output.string = result
                self.outputScroll.isHidden = success
                self.setState(
                    success ? (action == 2 ? "Finishing setup…" : "Action completed")
                        : (action == 2 ? "Setup was not completed" : "Action failed or authorization was cancelled"),
                    success ? (action == 2 ? "در حال تکمیل راه‌اندازی…" : "عملیات انجام شد")
                        : (action == 2 ? "راه‌اندازی کامل نشد" : "عملیات ناموفق بود یا تأیید لغو شد"),
                    color: success ? .labelColor : .systemOrange)
                // Do not log or copy the entire installation output. The copy
                // button copies only the newly issued panel credential.
                for line in result.components(separatedBy: .newlines) {
                    for prefix in ["first-run panel password: ", "New Caspian panel password: "] where line.hasPrefix(prefix) {
                        self.password = String(line.dropFirst(prefix.count))
                        self.copy.isEnabled = true
                        self.copy.isHidden = false
                        self.outputScroll.isHidden = false
                    }
                }
                self.resizeWindowForContent()
                if !success {
                    if action == 2 {
                        self.setDetail(
                            "Nothing was installed or updated. Select Set up Caspian when you are ready to approve the macOS administrator prompt.",
                            "چیزی نصب یا به‌روزرسانی نشد. وقتی آمادهٔ تأیید پنجرهٔ مدیر macOS بودید، راه‌اندازی کاسپین را بزنید.")
                    } else {
                        self.setDetail(
                            "No completed change was confirmed. You can retry from Advanced options.",
                            "هیچ تغییر کاملی تأیید نشد. می‌توانید از گزینه‌های پیشرفته دوباره تلاش کنید.")
                    }
                }
                if action == 2 && !success {
                    // Keep the cancellation/error explanation on screen. A
                    // fresh install check would immediately replace it with
                    // the generic welcome/update copy, hiding the useful
                    // reason and making a cancelled prompt look successful.
                    self.setupButton.isEnabled = true
                    self.serviceMenuItems.forEach { $0.isEnabled = false }
                    self.panelMenuItems.forEach { $0.isEnabled = false }
                } else {
                    self.check()
                }
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
appMenu.addItem(withTitle: "Quit Caspian Control / خروج از کنترل کاسپین", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
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
