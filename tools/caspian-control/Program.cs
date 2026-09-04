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
        Application.Run(new ControlWindow());
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
        ClientSize = new Size(620, 470);
        FormBorderStyle = FormBorderStyle.FixedDialog;
        MaximizeBox = false;
        StartPosition = FormStartPosition.CenterScreen;
        Font = new Font("Tahoma", 10);
        BackColor = Ground;
        Icon = brandIcon;

        var logo = new Logo { BackColor = Ground };
        logo.SetBounds(30, 15, 44, 44);
        var title = new Label {
            Text = "CASPIAN CONTROL", ForeColor = Teal,
            Font = new Font("Segoe UI Semibold", 11), TextAlign = ContentAlignment.MiddleLeft
        };
        title.SetBounds(84, 20, 506, 26);
        state.SetBounds(30, 62, 560, 70);
        state.BackColor = Surface;
        state.Font = new Font("Segoe UI Semibold", 24);
        state.TextAlign = ContentAlignment.MiddleCenter;
        details.SetBounds(30, 132, 560, 55);
        details.BackColor = Surface;
        details.ForeColor = Ink;
        details.TextAlign = ContentAlignment.TopCenter;

        Configure(start, "Start all", 30, Teal, Color.White, async (_, _) => await RunAsync(StartAll));
        Configure(stop, "Stop all", 175, Coral, Ink, async (_, _) => await RunAsync(StopAll));
        Configure(restart, "Restart all", 320, Amber, Ink, async (_, _) => await RunAsync(RestartAll));
        Configure(panel, "Open panel", 465, Teal, Color.White, (_, _) => OpenPanel());

        var description = new Label {
            Text = "Start all starts the Caspian engine and web panel.\r\n" +
                   "Restart all repairs stopped services. Open panel manages the hotspot, band, and tunnel.",
            ForeColor = Ink, BackColor = Surface, Padding = new Padding(18),
            TextAlign = ContentAlignment.MiddleLeft
        };
        description.SetBounds(30, 270, 560, 86);

        var about = new Panel { BackColor = Surface };
        about.SetBounds(30, 370, 560, 65);
        var aboutVersion = new Label {
            Text = $"Caspian v{DisplayVersion.TrimStart('v')}", ForeColor = Ink,
            Font = new Font("Segoe UI Semibold", 10), AutoSize = true, Location = new Point(16, 10)
        };
        var aboutDeveloper = new Label {
            Text = "توسعه‌دهنده: ایمان سمیع زاده", ForeColor = Ink,
            AutoSize = true, Location = new Point(160, 11), RightToLeft = RightToLeft.Yes
        };
        var aboutGitHub = new LinkLabel {
            Text = "پروژه در GitHub", LinkColor = Teal, ActiveLinkColor = Ink,
            AutoSize = true, Location = new Point(16, 37), RightToLeft = RightToLeft.Yes
        };
        aboutGitHub.LinkClicked += (_, _) => Process.Start(new ProcessStartInfo("https://github.com/Iman/caspian") { UseShellExecute = true });
        about.Controls.AddRange([aboutVersion, aboutDeveloper, aboutGitHub]);

        Controls.AddRange([logo, title, state, details, start, stop, restart, panel, description, about]);
        var menu = new ContextMenuStrip();
        menu.Items.Add("Open Caspian Control", null, (_, _) => ShowWindow());
        menu.Items.Add("Start all", null, async (_, _) => await RunAsync(StartAll));
        menu.Items.Add("Stop all", null, async (_, _) => await RunAsync(StopAll));
        menu.Items.Add("Restart all", null, async (_, _) => await RunAsync(RestartAll));
        menu.Items.Add("Open panel", null, (_, _) => OpenPanel());
        menu.Items.Add(new ToolStripSeparator());
        menu.Items.Add("Exit", null, (_, _) => { tray.Visible = false; Application.Exit(); });
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
        button.SetBounds(left, 205, 125, 48);
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
        state.Text = "Working…";
        state.ForeColor = Color.DarkOrange;
        try
        {
            await Task.Run(action).WaitAsync(TimeSpan.FromSeconds(45));
            await RefreshStateAsync(true);
        }
        catch (Exception ex)
        {
            state.Text = "Action failed";
            state.ForeColor = Color.Firebrick;
            details.Text = ex.Message;
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
            state.Text = ready ? "● Ready" : "● Not ready";
            state.ForeColor = ready ? Teal : Color.Firebrick;
            state.BackColor = ready ? Sage : Coral;
            tray.Text = ready ? "Caspian: Ready" : "Caspian: Not ready";
            details.Text = $"Engine: {Word(serviceState.Core)}    Panel service: {Word(serviceState.Web)}    Local panel: {Word(answering)}";
            panel.Enabled = ready;
        }
        catch
        {
            consecutiveFailures++;
            if (!immediateFailure && consecutiveFailures < 3) return;
            state.Text = "● Status timed out";
            state.ForeColor = Color.Firebrick;
            state.BackColor = Coral;
            details.Text = "The status request stopped. The window remains responsive.";
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
    internal void SaveScreenshot(string path)
    {
        StartPosition = FormStartPosition.Manual;
        Location = new Point(-10000, -10000);
        Show();
        Application.DoEvents();
        state.Text = "● Ready";
        state.ForeColor = Teal;
        state.BackColor = Sage;
        details.Text = "Engine: OK    Panel service: OK    Local panel: OK";
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
