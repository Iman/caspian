using System.Diagnostics;
using System.Drawing.Drawing2D;
using System.Net.Sockets;
using System.Reflection;
using System.Runtime.InteropServices;
using System.ServiceProcess;

namespace Caspian.Control;

internal static class Program
{
    [STAThread]
    private static void Main(string[] args)
    {
        ApplicationConfiguration.Initialize();
        if (args.Length == 1 && args[0] is "--start-all" or "--stop-all" or "--restart-all")
        {
            if (args[0] == "--start-all") ControlWindow.StartAll();
            if (args[0] == "--stop-all") ControlWindow.StopAll();
            if (args[0] == "--restart-all") ControlWindow.RestartAll();
            return;
        }
        if (args.Length == 2 && args[0] == "--export-icon")
        {
            Logo.SaveIcon(args[1]);
            return;
        }
        if (args.Length == 2 && args[0] == "--screenshot")
        {
            using var window = new ControlWindow();
            window.SaveScreenshot(args[1]);
            return;
        }
        using var instance = new SingleInstance();
        if (!instance.IsFirst)
        {
            instance.NotifyFirst();
            return;
        }
        using var mainWindow = new ControlWindow();
        var activation = instance.OnActivation(mainWindow, mainWindow.ShowAlreadyRunning);
        try { Application.Run(mainWindow); }
        finally { activation.Unregister(null); }
    }
}

internal sealed class ControlWindow : Form
{
    private static readonly string DisplayVersion =
        typeof(ControlWindow).Assembly.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?
            .InformationalVersion.Split('+')[0] ?? "dev";
    private readonly Label state = new();
    private readonly Label details = new();
    private readonly Button start = new();
    private readonly Button stop = new();
    private readonly Button restart = new();
    private readonly Button panel = new();
    private readonly NotifyIcon tray = new();
    private readonly Icon brandIcon = Logo.CreateIcon(32);
    private readonly System.Windows.Forms.Timer timer = new() { Interval = 3000 };
    private readonly SemaphoreSlim refreshLock = new(1, 1);
    private bool busy;
    private bool showingAlreadyRunning;
    private int consecutiveFailures;

    private static readonly Color Ground = ColorTranslator.FromHtml("#E6F2F3");
    private static readonly Color Surface = Color.White;
    private static readonly Color Ink = ColorTranslator.FromHtml("#05444A");
    private static readonly Color Teal = ColorTranslator.FromHtml("#097C87");
    private static readonly Color Sage = ColorTranslator.FromHtml("#A1CCA6");
    private static readonly Color Coral = ColorTranslator.FromHtml("#FCA47C");
    private static readonly Color Amber = ColorTranslator.FromHtml("#F9D779");

    internal ControlWindow()
    {
        Text = "Caspian Control";
        ClientSize = new Size(600, 390);
        FormBorderStyle = FormBorderStyle.FixedDialog;
        MaximizeBox = false;
        StartPosition = FormStartPosition.CenterScreen;
        Font = new Font("Tahoma", 9);
        BackColor = Ground;
        Icon = brandIcon;

        var logo = new Logo { BackColor = Ground };
        logo.SetBounds(20, 14, 34, 34);
        var title = new Label {
            Text = "CASPIAN CONTROL", ForeColor = Teal,
            Font = new Font("Segoe UI Semibold", 11), TextAlign = ContentAlignment.MiddleLeft
        };
        title.SetBounds(64, 10, 516, 44);
        title.Text += "\r\nکنترل کاسپین";
        state.SetBounds(20, 64, 560, 62);
        state.BackColor = Surface;
        state.Font = new Font("Tahoma", 15, FontStyle.Bold);
        state.TextAlign = ContentAlignment.MiddleCenter;
        details.SetBounds(20, 126, 560, 44);
        details.BackColor = Surface;
        details.ForeColor = Ink;
        details.TextAlign = ContentAlignment.MiddleCenter;

        Configure(panel, "Open panel\r\nباز کردن پنل", 20, Teal, Color.White, (_, _) => OpenPanel());
        Configure(start, "Start all\r\nراه‌اندازی همه", 164, Surface, Ink, async (_, _) => await RunAsync(StartAll));
        Configure(restart, "Restart all\r\nراه‌اندازی دوباره", 308, Surface, Ink, async (_, _) => await RunAsync(RestartAll));
        Configure(stop, "Stop all\r\nتوقف همه", 452, Surface, Ink, async (_, _) => await RunAsync(StopAll));

        var description = new Label {
            Text = "Open the panel to connect your tunnel and manage the hotspot.\r\n" +
                   "\u200fبرای اتصال تونل و مدیریت هات‌اسپات، پنل را باز کنید.",
            ForeColor = Ink, BackColor = Ground,
            TextAlign = ContentAlignment.MiddleCenter
        };
        description.SetBounds(20, 244, 560, 44);

        var about = new TableLayoutPanel {
            BackColor = Ground, ColumnCount = 3, RowCount = 1, GrowStyle = TableLayoutPanelGrowStyle.FixedSize,
            Padding = new Padding(0)
        };
        about.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 30));
        about.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 43));
        about.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 27));
        about.RowStyles.Add(new RowStyle(SizeType.Percent, 100));
        about.SetBounds(16, 302, 568, 68);
        var aboutVersion = new Label {
            Text = $"Version {VersionLabel}\r\n\u200fنسخه: \u200e{VersionLabel}", ForeColor = Ink, BackColor = Surface,
            Dock = DockStyle.Fill,
            TextAlign = ContentAlignment.MiddleCenter, Margin = new Padding(4, 0, 4, 0)
        };
        var aboutDeveloper = new Label {
            Text = "By Iman Samizadeh\r\nتوسعه‌دهنده: ایمان سمیع زاده", ForeColor = Ink, BackColor = Surface,
            Dock = DockStyle.Fill, TextAlign = ContentAlignment.MiddleCenter,
            Margin = new Padding(4, 0, 4, 0)
        };
        var aboutGitHub = new LinkLabel {
            Text = "GitHub project\r\nپروژه در گیت‌هاب", LinkColor = Teal, ActiveLinkColor = Ink, BackColor = Surface,
            Dock = DockStyle.Fill, TextAlign = ContentAlignment.MiddleCenter,
            Margin = new Padding(4, 0, 4, 0)
        };
        aboutGitHub.LinkClicked += (_, _) => Process.Start(new ProcessStartInfo("https://github.com/Iman/caspian") { UseShellExecute = true });
        about.Controls.Add(aboutVersion, 0, 0);
        about.Controls.Add(aboutDeveloper, 1, 0);
        about.Controls.Add(aboutGitHub, 2, 0);

        Controls.AddRange([logo, title, state, details, panel, start, restart, stop, description, about]);
        panel.TabIndex = 0;
        start.TabIndex = 1;
        restart.TabIndex = 2;
        stop.TabIndex = 3;
        var menu = new ContextMenuStrip();
        menu.Items.Add("Open Caspian Control / باز کردن کنترل کاسپین", null, (_, _) => ShowWindow());
        menu.Items.Add("Start all / راه‌اندازی همه", null, async (_, _) => await RunAsync(StartAll));
        menu.Items.Add("Stop all / توقف همه", null, async (_, _) => await RunAsync(StopAll));
        menu.Items.Add("Restart all / راه‌اندازی دوباره", null, async (_, _) => await RunAsync(RestartAll));
        menu.Items.Add("Open panel / باز کردن پنل", null, (_, _) => OpenPanel());
        menu.Items.Add(new ToolStripSeparator());
        menu.Items.Add("Exit / خروج", null, (_, _) => { tray.Visible = false; Application.Exit(); });
        tray.Icon = brandIcon;
        tray.Text = "Caspian Control";
        tray.ContextMenuStrip = menu;
        tray.Visible = true;
        tray.DoubleClick += (_, _) => ShowWindow();
        timer.Tick += async (_, _) => await RefreshStateAsync();
        Shown += async (_, _) => { timer.Start(); await RefreshStateAsync(true); };
        Resize += (_, _) => { if (WindowState == FormWindowState.Minimized) Hide(); };
        FormClosing += (_, e) =>
        {
            if (e.CloseReason != CloseReason.ApplicationExitCall)
            {
                e.Cancel = true;
                Hide();
            }
        };
    }

    private void Configure(Button button, string text, int left, Color background, Color foreground, EventHandler click)
    {
        button.Text = text;
        button.SetBounds(left, 182, 128, 54);
        button.FlatStyle = FlatStyle.Flat;
        button.FlatAppearance.BorderSize = 0;
        button.BackColor = background;
        button.ForeColor = foreground;
        button.Cursor = Cursors.Hand;
        button.Region = RoundedRegion(button.ClientRectangle, 9);
        button.Resize += (_, _) => button.Region = RoundedRegion(button.ClientRectangle, 9);
        button.Click += click;
    }

    private static ServiceController Service(string name) => new(name);

    private static void StartOne(string name)
    {
        using var service = Service(name);
        service.Refresh();
        if (service.Status == ServiceControllerStatus.Running) return;
        service.Start();
        service.WaitForStatus(ServiceControllerStatus.Running, TimeSpan.FromSeconds(30));
    }

    private static void StopOne(string name)
    {
        using var service = Service(name);
        service.Refresh();
        if (service.Status == ServiceControllerStatus.Stopped) return;
        service.Stop();
        service.WaitForStatus(ServiceControllerStatus.Stopped, TimeSpan.FromSeconds(30));
    }

    private static void SetAutomatic(string name, bool automatic)
    {
        using var process = Process.Start(new ProcessStartInfo
        {
            FileName = Path.Combine(Environment.SystemDirectory, "sc.exe"),
            Arguments = $"config {name} start= {(automatic ? "auto" : "disabled")}",
            UseShellExecute = false,
            CreateNoWindow = true
        }) ?? throw new InvalidOperationException("Windows could not update the Caspian service setting.");
        if (!process.WaitForExit(10_000) || process.ExitCode != 0)
            throw new InvalidOperationException($"Windows could not update the {name} service setting.");
    }

    internal static void StartAll()
    {
        SetAutomatic("caspian", true);
        SetAutomatic("caspian-panel", true);
        StartOne("caspian");
        StartOne("caspian-panel");
    }

    internal static void StopAll()
    {
        SetAutomatic("caspian-panel", false);
        SetAutomatic("caspian", false);
        StopOne("caspian-panel");
        StopOne("caspian");
    }
    internal static void RestartAll() { StopAll(); StartAll(); }

    private async Task RunAsync(Action action)
    {
        if (busy) return;
        busy = true;
        SetButtons(false);
        state.Text = "Working…\r\nدر حال انجام…";
        state.ForeColor = Color.DarkOrange;
        try
        {
            await Task.Run(action).WaitAsync(TimeSpan.FromSeconds(45));
            await RefreshStateAsync(true);
        }
        catch (Exception ex)
        {
            state.Text = "Action failed\r\nعملیات انجام نشد";
            state.ForeColor = Color.Firebrick;
            details.Text = ex.Message + "\r\nعملیات انجام نشد. وضعیت سرویس‌ها را بررسی کنید و دوباره تلاش کنید.";
        }
        finally { busy = false; SetButtons(true); }
    }

    private async Task RefreshStateAsync(bool immediateFailure = false)
    {
        if (busy || !await refreshLock.WaitAsync(0)) return;
        try
        {
            var serviceState = await Task.Run(() => (Core: IsRunning("caspian"), Web: IsRunning("caspian-panel")))
                .WaitAsync(TimeSpan.FromSeconds(3));
            bool answering = serviceState.Web && await PortAnswersAsync(8088);
            bool ready = serviceState.Core && serviceState.Web && answering;
            if (ready)
            {
                consecutiveFailures = 0;
            }
            else
            {
                consecutiveFailures++;
                if (!immediateFailure && consecutiveFailures < 3) return;
            }
            state.Text = ready ? "● Ready\r\nآماده" : "● Not ready\r\nآماده نیست";
            state.ForeColor = ready ? Teal : Color.Firebrick;
            state.BackColor = ready ? Sage : Coral;
            tray.Text = ready ? "Caspian: Ready" : "Caspian: Not ready";
            details.Text = StatusDetails(serviceState.Core, serviceState.Web, answering);
            panel.Enabled = ready;
        }
        catch
        {
            consecutiveFailures++;
            if (!immediateFailure && consecutiveFailures < 3) return;
            state.Text = "Status timed out\r\nمهلت بررسی وضعیت تمام شد";
            state.ForeColor = Color.Firebrick;
            state.BackColor = Coral;
            details.Text = "The status request timed out. Please try again.\r\nمهلت بررسی وضعیت تمام شد. دوباره تلاش کنید.";
        }
        finally { refreshLock.Release(); }
    }

    protected override void Dispose(bool disposing)
    {
        if (disposing)
        {
            timer.Dispose();
            tray.Dispose();
            brandIcon.Dispose();
            refreshLock.Dispose();
        }
        base.Dispose(disposing);
    }

    private static bool IsRunning(string name)
    {
        try { using var s = Service(name); return s.Status == ServiceControllerStatus.Running; }
        catch { return false; }
    }

    private static async Task<bool> PortAnswersAsync(int port)
    {
        try
        {
            using var client = new TcpClient();
            await client.ConnectAsync("127.0.0.1", port).WaitAsync(TimeSpan.FromSeconds(2));
            return true;
        }
        catch { return false; }
    }

    private static string Word(bool value) => value ? "OK" : "OFF";
    private static string PersianWord(bool value) => value ? "فعال" : "غیرفعال";
    private static string VersionLabel => DisplayVersion == "dev" ? "dev" : "v" + DisplayVersion.TrimStart('v');
    private static string StatusDetails(bool core, bool web, bool answering) =>
        $"Engine service: {Word(core)}    Panel service: {Word(web)}    Local panel: {Word(answering)}\r\n" +
        $"\u200fسرویس موتور: {PersianWord(core)}    سرویس پنل: {PersianWord(web)}    پنل محلی: {PersianWord(answering)}";
    internal void SaveScreenshot(string path)
    {
        StartPosition = FormStartPosition.Manual;
        Location = new Point(-10000, -10000);
        Show();
        Application.DoEvents();
        state.Text = "● Ready\r\nآماده";
        state.ForeColor = Teal;
        state.BackColor = Sage;
        details.Text = StatusDetails(true, true, true);
        panel.Enabled = true;
        using var bitmap = new Bitmap(Width, Height);
        DrawToBitmap(bitmap, new Rectangle(Point.Empty, Size));
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
        bitmap.Save(path, System.Drawing.Imaging.ImageFormat.Png);
        tray.Visible = false;
        Hide();
    }
    private static Region RoundedRegion(Rectangle bounds, int radius)
    {
        using var path = new GraphicsPath();
        int diameter = radius * 2;
        path.AddArc(0, 0, diameter, diameter, 180, 90);
        path.AddArc(bounds.Width - diameter, 0, diameter, diameter, 270, 90);
        path.AddArc(bounds.Width - diameter, bounds.Height - diameter, diameter, diameter, 0, 90);
        path.AddArc(0, bounds.Height - diameter, diameter, diameter, 90, 90);
        path.CloseFigure();
        return new Region(path);
    }
    private void ShowWindow() { Show(); WindowState = FormWindowState.Normal; Activate(); }
    internal void ShowAlreadyRunning()
    {
        ShowWindow();
        if (showingAlreadyRunning) return;
        showingAlreadyRunning = true;
        try
        {
            MessageBox.Show(this,
                "Caspian is already running. Use this window to manage it.\r\n\r\n" +
                "کاسپین از قبل در حال اجراست. برای مدیریت آن از همین پنجره استفاده کنید.",
                "Caspian / کاسپین", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }
        finally { showingAlreadyRunning = false; }
    }
    private void SetButtons(bool enabled) { start.Enabled = enabled; stop.Enabled = enabled; restart.Enabled = enabled; }
    private static void OpenPanel() => Process.Start(new ProcessStartInfo("http://127.0.0.1:8088/") { UseShellExecute = true });
}

internal sealed class Logo : System.Windows.Forms.Control
{
    protected override void OnPaint(PaintEventArgs e)
    {
        base.OnPaint(e);
        Draw(e.Graphics, ClientRectangle);
    }

    internal static Icon CreateIcon(int size)
    {
        using var bitmap = new Bitmap(size, size);
        using (var graphics = Graphics.FromImage(bitmap)) Draw(graphics, new Rectangle(0, 0, size, size));
        var handle = bitmap.GetHicon();
        try { return (Icon)Icon.FromHandle(handle).Clone(); }
        finally { DestroyIcon(handle); }
    }

    internal static void SaveIcon(string path)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
        using var icon = CreateIcon(256);
        using var stream = File.Create(path);
        icon.Save(stream);
    }

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool DestroyIcon(IntPtr handle);

    private static void Draw(Graphics graphics, Rectangle area)
    {
        graphics.SmoothingMode = SmoothingMode.AntiAlias;
        float scale = Math.Min(area.Width, area.Height) / 32f;
        graphics.TranslateTransform(area.Left, area.Top);
        graphics.ScaleTransform(scale, scale);
        using var background = new SolidBrush(ColorTranslator.FromHtml("#05444A"));
        using var shield = new Pen(ColorTranslator.FromHtml("#A1CCA6"), 2.2f) { LineJoin = LineJoin.Round };
        using var signal = new Pen(ColorTranslator.FromHtml("#23CED9"), 2f) { StartCap = LineCap.Round, EndCap = LineCap.Round };
        using var dot = new SolidBrush(ColorTranslator.FromHtml("#23CED9"));
        graphics.FillRoundedRectangle(background, new RectangleF(0, 0, 32, 32), 7);
        using var shieldPath = new GraphicsPath();
        shieldPath.AddLines([new PointF(16, 5), new PointF(25, 9), new PointF(25, 16)]);
        shieldPath.AddBezier(25, 16, 25, 21.5f, 21.3f, 25.6f, 16, 27.4f);
        shieldPath.AddBezier(16, 27.4f, 10.7f, 25.6f, 7, 21.5f, 7, 16);
        shieldPath.AddLines([new PointF(7, 16), new PointF(7, 9), new PointF(16, 5)]);
        graphics.DrawPath(shield, shieldPath);
        graphics.DrawArc(signal, 11.5f, 13.2f, 9, 7, 205, 130);
        graphics.FillEllipse(dot, 14.3f, 18.7f, 3.4f, 3.4f);
        graphics.ResetTransform();
    }
}

internal static class GraphicsExtensions
{
    internal static void FillRoundedRectangle(this Graphics graphics, Brush brush, RectangleF rectangle, float radius)
    {
        using var path = new GraphicsPath();
        float diameter = radius * 2;
        path.AddArc(rectangle.Left, rectangle.Top, diameter, diameter, 180, 90);
        path.AddArc(rectangle.Right - diameter, rectangle.Top, diameter, diameter, 270, 90);
        path.AddArc(rectangle.Right - diameter, rectangle.Bottom - diameter, diameter, diameter, 0, 90);
        path.AddArc(rectangle.Left, rectangle.Bottom - diameter, diameter, diameter, 90, 90);
        path.CloseFigure();
        graphics.FillPath(brush, path);
    }
}
