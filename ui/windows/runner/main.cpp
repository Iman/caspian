#include <flutter/dart_project.h>
#include <flutter/flutter_view_controller.h>
#include <windows.h>
#include <sddl.h>

#include <cstdlib>
#include <string>
#include <vector>

#include "flutter_window.h"
#include "utils.h"

namespace {

// Local objects stay in this sign-in session. The SID also separates users
// sharing a session. The token's default DACL controls access to both objects.
std::wstring InstanceName() {
  HANDLE token = nullptr;
  if (!::OpenProcessToken(::GetCurrentProcess(), TOKEN_QUERY, &token)) {
    return {};
  }
  DWORD size = 0;
  ::GetTokenInformation(token, TokenUser, nullptr, 0, &size);
  if (size == 0 || ::GetLastError() != ERROR_INSUFFICIENT_BUFFER) {
    ::CloseHandle(token);
    return {};
  }
  std::vector<BYTE> information(size);
  const BOOL read = ::GetTokenInformation(token, TokenUser, information.data(),
                                          size, &size);
  ::CloseHandle(token);
  if (!read) {
    return {};
  }
  const auto* user = reinterpret_cast<const TOKEN_USER*>(information.data());
  LPWSTR sid = nullptr;
  if (!::ConvertSidToStringSidW(user->User.Sid, &sid)) {
    return {};
  }
  std::wstring name = L"Local\\Caspian.UI.";
  name += sid;
  ::LocalFree(sid);
  return name;
}

class SingleInstance {
 public:
  SingleInstance() {
    const auto name = InstanceName();
    if (name.empty()) {
      return;
    }
    // Create the event before contending for the mutex. A second launch can
    // signal it before the first window exists; that signal remains pending.
    activation_ = ::CreateEventW(nullptr, FALSE, FALSE,
                                  (name + L".Activate").c_str());
    if (!activation_) {
      return;
    }
    mutex_ = ::CreateMutexW(nullptr, FALSE, (name + L".Mutex").c_str());
    if (!mutex_) {
      return;
    }
    const DWORD result = ::WaitForSingleObject(mutex_, 0);
    first_ = result == WAIT_OBJECT_0 || result == WAIT_ABANDONED;
    valid_ = first_ || result == WAIT_TIMEOUT;
  }

  ~SingleInstance() {
    if (first_) {
      ::ReleaseMutex(mutex_);
    }
    if (mutex_) {
      ::CloseHandle(mutex_);
    }
    if (activation_) {
      ::CloseHandle(activation_);
    }
  }

  SingleInstance(const SingleInstance&) = delete;
  SingleInstance& operator=(const SingleInstance&) = delete;

  bool valid() const { return valid_; }
  bool first() const { return first_; }
  bool ActivateFirst() const { return ::SetEvent(activation_) != FALSE; }
  HANDLE activation() const { return activation_; }

 private:
  HANDLE mutex_ = nullptr;
  HANDLE activation_ = nullptr;
  bool first_ = false;
  bool valid_ = false;
};

int RunMessageLoop(FlutterWindow& window, HANDLE activation) {
  while (true) {
    MSG message;
    while (::PeekMessageW(&message, nullptr, 0, 0, PM_REMOVE)) {
      if (message.message == WM_QUIT) {
        return EXIT_SUCCESS;
      }
      ::TranslateMessage(&message);
      ::DispatchMessageW(&message);
    }
    const DWORD result = ::MsgWaitForMultipleObjects(
        1, &activation, FALSE, INFINITE, QS_ALLINPUT);
    if (result == WAIT_OBJECT_0) {
      HWND handle = window.GetHandle();
      if (::IsWindow(handle)) {
        ::ShowWindow(handle, ::IsIconic(handle) ? SW_RESTORE : SW_SHOW);
        ::SetForegroundWindow(handle);
      }
    } else if (result != WAIT_OBJECT_0 + 1) {
      return EXIT_FAILURE;
    }
  }
}

}  // namespace

int APIENTRY wWinMain(_In_ HINSTANCE instance, _In_opt_ HINSTANCE prev,
                      _In_ wchar_t* command_line, _In_ int show_command) {
  // Keep both handles, and mutex ownership, until the window has been destroyed.
  SingleInstance single_instance;
  if (!single_instance.valid()) {
    ::MessageBoxW(nullptr, L"Caspian could not open its application session.",
                  L"Caspian", MB_OK | MB_ICONERROR);
    return EXIT_FAILURE;
  }
  if (!single_instance.first()) {
    return single_instance.ActivateFirst() ? EXIT_SUCCESS : EXIT_FAILURE;
  }

  if (!::AttachConsole(ATTACH_PARENT_PROCESS) && ::IsDebuggerPresent()) {
    CreateAndAttachConsole();
  }
  if (FAILED(::CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED))) {
    return EXIT_FAILURE;
  }

  int result = EXIT_FAILURE;
  {
    flutter::DartProject project(L"data");
    project.set_dart_entrypoint_arguments(GetCommandLineArguments());
    FlutterWindow window(project);
    Win32Window::Point origin(10, 10);
    Win32Window::Size size(1280, 720);
    if (window.Create(L"Caspian", origin, size)) {
      // Closing the GUI never sends a stop command to the gateway service.
      window.SetQuitOnClose(true);
      result = RunMessageLoop(window, single_instance.activation());
    }
  }
  ::CoUninitialize();
  return result;
}
