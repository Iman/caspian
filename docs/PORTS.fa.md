# پورت‌های macOS و Windows: چه ساخته شده، چه اندازه‌گیری شده، چه نشده

🇮🇷 **فارسی** | [🇬🇧 English](PORTS.md) | [🇷🇺 Русский](../README.ru.md) | [🇨🇳 中文](../README.zh.md)

> نسخهٔ انگلیسی: [`docs/PORTS.md`](PORTS.md). آزمون‌ها نسخهٔ انگلیسی را
> می‌خوانند و مرجع همان است. اگر این دو با هم اختلاف داشتند، انگلیسی درست است.

شاخهٔ `port/platforms`، شروع‌شده در 2026-09-03. این فایل وضعیت صادقانهٔ دو
پورتِ دسکتاپ است. هر چیزی که با MEASURED نشانه‌گذاری شده روی یک ماشین اجرا
شده؛ هر چیزی با VERIFIED در یک منبع دست‌اول خوانده شده (مستندات Apple یا
Microsoft، باینری‌های خودِ سیستم‌عامل، یا سورس کتابخانه‌های نام‌برده)؛
UNVERIFIED یعنی هیچ‌کدام، و در این پروژه هیچ چیزی حق ندارد بدون گفتنِ آن، بر
یک ادعای UNVERIFIED تکیه کند.

دستگاه لینوکسی تغییری نکرده است. هر فرمانی که اجرا می‌کند، هر قاعده‌ای که تولید
می‌کند و هر آزمونی که داشت، بایت به بایت همان است؛ پورت‌ها با گذاشتن یک درز زیر
آن ساخته شده‌اند، نه با ویرایش آن.

## درزها

| درز | کجا | چه چیزی در هر پلتفرم فرق می‌کند |
|---|---|---|
| بک‌اندِ شبکه | `internal/netcfg/platform.go`، `Backend` | ماشین چگونه خوانده می‌شود، طرح به چه تبدیل می‌شود، چگونه بازخوانی می‌شود. با `Options.Platform` انتخاب می‌شود، یک مقدار، پس هر سه همه‌جا کامپایل و آزمون می‌شوند |
| اجراکننده | `exec_linux.go`، `exec_darwin.go`، `exec_windows.go` | تنها کدِ با برچسب بیلد: آنچه واقعاً اجرا می‌شود. لینوکس و macOS باینری‌های مسیر ثابت را از یک فهرست بستهٔ مجاز اجرا می‌کنند؛ Windows در همان پروسه IP Helper API و WFP و Wintun را صدا می‌زند، به شکل شبه‌باینری، تا ژورنال و قواعد idempotence مشترک بمانند |
| نقطهٔ دسترسی | `internal/hotspot/accesspoint.go`، `AccessPoint` | hostapd و dnsmasq زیر Supervisor (لینوکس)، Internet Sharing اپل (`internetsharing.go`)، Mobile Hotspot از راه یک کمک‌برنامه (`mobilehotspot.go`) |
| حمل‌ونقل | `internal/privsvc/transport_unix.go`، `transport_windows.go` | سوکت unix با اعتبارِ همتا از هسته، یا named pipe با توصیف‌گر امنیتی و impersonation |
| چیدمان | `cmd/caspian/paths_*.go` | مسیرها، حساب سرویس، فعل‌های مدیر سرویس |
| امتیاز و چرخهٔ حیات | `cmd/caspian/privilege_*.go`، `lifecycle_*.go` | euid صفر در برابر توکنِ elevated؛ سیگنال‌ها در برابر Service Control Manager |

## macOS

در 2026-09-03 با مالک تصمیم گرفته شد: نقطهٔ دسترسی رادیوی خودِ مک از راه
Internet Sharing اپل است، و اینترنت از یک رابطِ سیمی می‌آید (Ethernet،
USB Ethernet، iPhone USB). یک آداپتور Wi-Fi USB نمی‌تواند روی Apple Silicon
نقطهٔ دسترسی باشد: هیچ سازنده‌ای برای رادیوهای USB رئال‌تک یا مدیاتک درایور
macOS 11 یا بالاتر منتشر نکرده، DriverKit خانوادهٔ Wi-Fi ندارد، macOS هیچ API
که یک رادیو را در حالت نقطهٔ دسترسی بگذارد ندارد، و hostapd بک‌اند Darwin ندارد
(همه VERIFIED؛ منابع در پژوهش پورت). یک رادیو نمی‌تواند هم‌زمان station و
نقطهٔ دسترسی باشد، پس مکی که تنها اینترنتش Wi-Fi خودش است نمی‌تواند میزبان
شود؛ برنامه‌ریز این را با کلمات می‌گوید.

MEASURED روی مکِ توسعه (macOS 26.6، Apple Silicon)، فقط‌خواندنی:

- `caspian check` تشخیصِ macOS را سر تا سر اجرا می‌کند و برنامه‌ریز وقتی رادیوی
  Wi-Fi همان uplink است درست رد می‌کند.
- Internet Sharing یک پلاگین configd است (`InternetSharingPreference.bundle`)
  که `com.apple.nat.plist` را از راه SCPreferences می‌پاید، به‌اضافهٔ دیمنِ XPC
  `com.apple.NetworkSharing`؛ دیگر هیچ کار launchd با نام
  `com.apple.InternetSharing` وجود ندارد. bootpd کارِ DHCP را می‌کند؛ پروکسیِ
  mDNSResponder به DNS کلاینت‌ها جواب می‌دهد.

ساخته و آزمون واحد شده (با recorderها، بدون root):

- `darwinnet_detect.go`: `ifconfig -a`، `route -n get default`،
  `networksetup -listallhardwareports` و `-getairportnetwork`،
  `sysctl -e`، `pfctl -s info`، به درونِ `Facts` مشترک.
- `darwinnet_steps.go`: یک anchor در pf با نام `com.apple/250.CaspianBYOC`
  (که `anchor "com.apple/*"` در `/etc/pf.conf` بدون ویرایشِ قواعد اصلی آن را
  ارزیابی می‌کند)، در یک تراکنش بار می‌شود. DNS کلاینت را به موتور هدایت می‌کند،
  ترافیک کلاینت را با `route-to` به درون utun می‌راند، هر چیز دیگری از کلاینت‌ها
  را در ورودِ `bridge100` مسدود می‌کند، و پنل را می‌گذارد بگذرد. مسیرِ سنجاق‌شده
  به سرور با `route -n add -host`. `net.inet.ip.forwarding` روشن. گامِ نشانیِ
  تونل نداریم: TUN داروینِ xray-core خودش 169.254.10.2/30 را می‌گذارد و اصرار
  دارد دستگاه `utunN` باشد، پس دستگاه `utun100` است.
- `internetsharing.go`: فایل تنظیمات را با کلیدهایی می‌نویسد که دامپ‌های واقعی
  و رشته‌های پلاگین نشان می‌دهند (NetworkName، NetworkPassword به شکل دادهٔ
  UTF-16LE، Channel، PrimaryService به عنوان UUID سرویسِ uplink،
  SharingDevices، SharingNetworkNumberStart/End/Mask از زیرشبکهٔ طرح)، آن را از
  راه `scutil --prefs` دوباره ذخیره می‌کند تا configd اعلان‌های commit و apply را
  بگیرد، اگر bridge ظاهر نشد یک بار دیمن را kickstart می‌کند، و bridge را
  بازمی‌خواند. شمار دستگاه‌ها از `/var/db/dhcpd_leases`.

UNVERIFIED تا وقتی با root روی یک مک اجرا شود، به این ترتیب (اسکریپت:
`local/measure-internet-sharing.sh`، در gitignore، پیش از ذخیره پاک‌سازی می‌کند):

1. اینکه پنجرهٔ Sharing در macOS 26 کلیدهای بالا را به همان شکل می‌نویسد، و رمزِ
   WPA را کجا نگه می‌دارد (در plist، یا در یک آیتمِ System keychain از راه API
   خصوصی HostAP در CoreWLAN). اگر keychain، درایور یک گامِ
   `security add-generic-password` با ویژگی‌های اندازه‌گیری‌شده نیاز دارد.
2. اینکه commit و apply در `scutil --prefs` روی 26.6 اشتراک را شروع می‌کند، یا
   `launchctl kickstart -k system/com.apple.NetworkSharing` می‌کند.
3. اینکه `route-to` در pf روی utun در pf اپل کار می‌کند (`pf_route` در xnu می‌گوید
   کار می‌کند؛ هیچ‌کس اجرایی منتشر نکرده)، و اینکه یک `rdr` در فرزندِ `com.apple/*`
   برای ترافیک `bridge100` رعایت می‌شود.
4. اینکه Internet Sharing با دیدنِ utun خودش را جمع نمی‌کند.

اجرای آن روی این مک یک uplink اترنت می‌خواهد (یک کابل در یکی از آداپتورهای
USB Ethernet)، `sudo`، و `bash packaging/darwin/install-darwin.sh`.

## Windows

در 2026-09-03 با مالک تصمیم گرفته شد: Mobile Hotspot نقطهٔ دسترسی است (با
کمک‌برنامهٔ C# در `tools/caspian-tethering`)، کلِ میزبان تونل می‌شود چون Windows
مسیریابیِ بر پایهٔ مبدأ ندارد، و fail-closed با فیلترهای Windows Filtering
Platform در لایهٔ forwarding اعمال می‌شود، چون قواعد فایروال معمولی هرگز ترافیک
forward شده را نمی‌بینند.

اینجا ساخته و برای `windows/amd64` و `windows/arm64` کامپایل‌چک شده؛ کمک‌برنامهٔ
C# روی این مک در برابر projection واقعی WinRT کامپایل می‌شود:

- `winnet.go`: بک‌اند. شبه‌باینری‌های `iphlpapi`، `wfp`، `wintun`. پیش از موتور:
  ساختِ آداپتور تونل (تا فیلترها بتوانند نامش را ببرند)، بارگذاری فیلترها،
  forwarding روشن، مسیرِ سنجاق‌شدهٔ میزبان به سرور. پس از موتور: نشانی، متریکِ رابط
  صفر، مسیرِ پیش‌فرض از تونل. زیرشبکهٔ هات‌اسپات به 192.168.137.0/24 سنجاق شده،
  که Internet Connection Sharing سرو می‌کند و نمی‌شود جور دیگری به آن گفت.
- `exec_windows.go` و `winsys_windows.go`: اجراکننده. ساختارها فیلد به فیلد از
  بسته‌های MIT `winipcfg` و `firewall` در WireGuard کپی شده‌اند؛ GUIDهای لایه و
  شرطِ WFP از هدرهای Microsoft به واسطهٔ tailscale/wf. فیلترها ماندگارند (از
  پروسه بیشتر عمر می‌کنند، پس یک crash جعبه را بسته می‌گذارد) زیر یک provider و
  sublayer با کلیدهای ثابت.
- `mobilehotspot.go` و کمک‌برنامه: یک پروسه برای هر عمل، JSON در ورودی، یک خط
  JSON در خروجی. کمک‌برنامه توانِ tethering را می‌سنجد و دلیلِ Windows را با نام
  می‌گزارد، SSID و رمز و باند را اعمال می‌کند، مهلتِ پنج‌دقیقه‌ایِ بی‌کلاینت را
  خاموش می‌کند، شروع می‌کند، و حالت را بازمی‌خواند.
- `transport_windows.go`: named pipe در `\\.\pipe\caspian-priv`، توصیف‌گر امنیتی
  که SYSTEM، Administrators و حساب سرویسِ مجازیِ پنل را می‌پذیرد؛ SID کلاینت از
  راه impersonation خوانده می‌شود.
- `packaging/windows/install.ps1`: دو سرویس، ACLها، رمزِ نخستین اجرا.

برای تمام کردن روی ماشین Windows، به این ترتیب:

1. ساخت: `go build -o caspian.exe ./cmd/caspian` و
   `dotnet publish -c Release -r win-x64 -o out tools/caspian-tethering`؛
   `wintun.dll` را از wintun.net بگیرید (نامش را تغییر ندهید).
2. `caspian.exe check` از یک پرامپتِ elevated: فهرست آداپتورها، طرح.
3. کمک‌برنامه با دست: `echo {"op":"status","uplink":"Ethernet"} |
   caspian-tethering.exe status`، سپس یک start با نام‌بردنِ دانگل به عنوان
   آداپتور. این تعیین می‌کند که overload `(profile, NetworkAdapter)` دانگل را
   سنجاق می‌کند یا نه و اینکه یک پروفایلِ Wintun بی‌اینترنت پذیرفته می‌شود یا نه.
4. اینکه بسته‌های کلاینت پس از ترجمهٔ ICS جدول مسیریابی میزبان را به درون تونل
   دنبال می‌کنند (باید، با مسیر پیش‌فرض روی آن) یا از آداپتورِ اشتراک‌گذاشته
   بیرون رانده می‌شوند. اگر رانده شدند، جایگزین اشتراک از پروفایلِ آداپتور Wintun
   است، که می‌شود به کمک‌برنامه گفت.
5. اینکه فیلترهای forwarding در WFP بسته‌های کلاینت را می‌بینند (پیش یا پس از
   ترجمهٔ ICS) و وقتی تونل رفته آن‌ها را می‌اندازند.
6. DNS کلاینت: به کلاینت‌ها 192.168.137.1 گفته می‌شود و پروکسیِ ICS با
   resolverهای میزبان جواب می‌دهد، که حالا از تونل می‌روند؛ با یک capture روی
   uplink تأیید کنید که هیچ پورت 53 ای به شکل روشن بیرون نمی‌رود.

## هنوز روی هیچ پلتفرمی انجام نشده

- xray-core همچنان v1.260327.0 است. v26.4.15 متد `Handler.Close` را می‌افزاید که
  موتور را وادار می‌کند دستگاه TUN را روی هر پلتفرمی آزاد کند و اجازه می‌دهد مسیرِ
  آزادسازیِ فقط‌لینوکسی در `internal/engine/tundevice_linux.go` برود. این تغییرِ
  نسخهٔ سنجاق‌شده‌ای است که README مستند کرده، پس تصمیمی جداست.
- `docs/LAYOUT.md` تنها جدولِ لینوکس را مستند می‌کند. `cmd/caspian/paths_*.go`
  جدول‌های macOS و Windows را دارند؛ سند باید هر دو را بگیرد.
- `install.sh` همچنان هر چیزی که لینوکس نیست را رد می‌کند، به قصد؛ نصاب‌های
  پلتفرمی زیر `packaging/darwin` و `packaging/windows` هستند.
- روی Windows حفاظتِ 0600 که `internal/state` قول می‌دهد در کار نیست
  (`perm_other.go` این را می‌گوید)؛ ACLهای نصاب جای آن را می‌گیرند.
