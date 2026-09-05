using System.Diagnostics;
using Caspian.Control;

internal static class Program
{
    [STAThread]
    private static int Main(string[] args)
    {
        if (args.Length == 2 && args[0] == "--contend")
        {
            using var contender = new SingleInstance(args[1]);
            if (contender.IsFirst) return 2;
            contender.NotifyFirst();
            return 0;
        }
        ApplicationConfiguration.Initialize();
        foreach (bool duringStartup in new[] { false, true })
        {
            string name = "Caspian.Control.Test." + Guid.NewGuid().ToString("N");
            using (var first = new SingleInstance(name))
            {
                if (!first.IsFirst) throw new Exception("First process did not acquire the mutex.");
                using var control = new Control(); // Invisible handle for the UI callback.
                bool activated = false;
                using var timeout = new System.Windows.Forms.Timer { Interval = 5000 };
                timeout.Tick += (_, _) => Application.ExitThread();
                Process? second = duringStartup ? Launch(name) : null;
                if (second is not null && !second.WaitForExit(5000)) throw new Exception("Second process did not exit.");
                var registration = first.OnActivation(control, () => { activated = true; Application.ExitThread(); });
                try
                {
                    second ??= Launch(name);
                    timeout.Start();
                    Application.Run();
                    if (!second.WaitForExit(5000) || second.ExitCode != 0)
                        throw new Exception("Second process acquired ownership or failed to exit.");
                    if (!activated) throw new Exception("Existing instance was not notified.");
                }
                finally { registration.Unregister(null); second?.Dispose(); }
            }
            using var reopened = new SingleInstance(name);
            if (!reopened.IsFirst) throw new Exception("Closing the first instance did not release ownership.");
            Console.WriteLine($"PASS: second launch {(duringStartup ? "during startup" : "while running")} activates first and exits; reopening succeeds.");
        }
        return 0;
    }

    private static Process Launch(string name)
    {
        var info = new ProcessStartInfo(Environment.ProcessPath!) { UseShellExecute = false, CreateNoWindow = true };
        if (string.Equals(Path.GetFileNameWithoutExtension(Environment.ProcessPath), "dotnet", StringComparison.OrdinalIgnoreCase))
            info.ArgumentList.Add(typeof(Program).Assembly.Location);
        info.ArgumentList.Add("--contend");
        info.ArgumentList.Add(name);
        return Process.Start(info) ?? throw new Exception("Could not launch contender.");
    }
}
