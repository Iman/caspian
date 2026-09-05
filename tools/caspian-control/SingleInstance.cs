namespace Caspian.Control;

// Local names keep activation in the same Windows sign-in session.
internal sealed class SingleInstance : IDisposable
{
    private readonly EventWaitHandle activation;
    private readonly Mutex mutex;
    internal bool IsFirst { get; }

    internal SingleInstance(string name = "Caspian.Control")
    {
        activation = new EventWaitHandle(false, EventResetMode.AutoReset, @"Local\" + name + ".Activate");
        mutex = new Mutex(false, @"Local\" + name);
        try { IsFirst = mutex.WaitOne(0); }
        catch (AbandonedMutexException) { IsFirst = true; }
    }

    internal void NotifyFirst() => activation.Set();

    internal RegisteredWaitHandle OnActivation(System.Windows.Forms.Control window, Action onActivation)
    {
        // Create the handle before registering. A launch during startup leaves
        // the event signalled and is handled once the message loop is running.
        _ = window.Handle;
        return ThreadPool.RegisterWaitForSingleObject(activation, (_, _) =>
        {
            try { window.BeginInvoke(onActivation); }
            catch (InvalidOperationException) { /* The existing window is closing. */ }
        }, null, Timeout.Infinite, false);
    }

    public void Dispose()
    {
        if (IsFirst) mutex.ReleaseMutex();
        mutex.Dispose();
        activation.Dispose();
    }
}
