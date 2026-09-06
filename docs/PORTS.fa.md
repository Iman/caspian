<div dir="rtl" align="right">

# پورت‌های macOS و Windows: چه ساخته شده، چه اندازه‌گیری شده، چه نشده

🇮🇷 **فارسی** | [🇬🇧 English](PORTS.md) | [🇷🇺 Русский](../README.ru.md) | [🇨🇳 中文](../README.zh.md)

> نسخهٔ انگلیسی: [<span dir="ltr">`docs/PORTS.md`</span>](PORTS.md). آزمون‌ها نسخهٔ انگلیسی را
> می‌خوانند و مرجع همان است. اگر این دو با هم اختلاف داشتند، انگلیسی درست است.

شاخهٔ <span dir="ltr">`port/platforms`</span>، شروع‌شده در 2026-09-03. این فایل وضعیت صادقانهٔ دو
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
| بک‌اندِ شبکه | <span dir="ltr">`internal/netcfg/platform.go`</span>، <span dir="ltr">`Backend`</span> | ماشین چگونه خوانده می‌شود، طرح به چه تبدیل می‌شود، چگونه بازخوانی می‌شود. با <span dir="ltr">`Options.Platform`</span> انتخاب می‌شود، یک مقدار، پس هر سه همه‌جا کامپایل و آزمون می‌شوند |
| اجراکننده | <span dir="ltr">`exec_linux.go`</span>، <span dir="ltr">`exec_darwin.go`</span>، <span dir="ltr">`exec_windows.go`</span> | تنها کدِ با برچسب بیلد: آنچه واقعاً اجرا می‌شود. لینوکس و macOS باینری‌های مسیر ثابت را از یک فهرست بستهٔ مجاز اجرا می‌کنند؛ Windows در همان پروسه IP Helper API و WFP و Wintun را صدا می‌زند، به شکل شبه‌باینری، تا ژورنال و قواعد idempotence مشترک بمانند |
| نقطهٔ دسترسی | <span dir="ltr">`internal/hotspot/accesspoint.go`</span>، <span dir="ltr">`AccessPoint`</span> | hostapd و dnsmasq زیر Supervisor (لینوکس)، Internet Sharing اپل (<span dir="ltr">`internetsharing.go`</span>)، Mobile Hotspot از راه یک کمک‌برنامه (<span dir="ltr">`mobilehotspot.go`</span>) |
| حمل‌ونقل | <span dir="ltr">`internal/privsvc/transport_unix.go`</span>، <span dir="ltr">`transport_windows.go`</span> | سوکت unix با اعتبارِ همتا از هسته، یا named pipe با توصیف‌گر امنیتی و impersonation |
| چیدمان | <span dir="ltr">`cmd/caspian/paths_*.go`</span> | مسیرها، حساب سرویس، فعل‌های مدیر سرویس |
| امتیاز و چرخهٔ حیات | <span dir="ltr">`cmd/caspian/privilege_*.go`</span>، <span dir="ltr">`lifecycle_*.go`</span> | euid صفر در برابر توکنِ elevated؛ سیگنال‌ها در برابر Service Control Manager |

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

- <span dir="ltr">`caspian check`</span> تشخیصِ macOS را سر تا سر اجرا می‌کند و برنامه‌ریز وقتی رادیوی
  Wi-Fi همان uplink است درست رد می‌کند.
- Internet Sharing یک پلاگین configd است (<span dir="ltr">`InternetSharingPreference.bundle`</span>)
  که <span dir="ltr">`com.apple.nat.plist`</span> را از راه SCPreferences می‌پاید، به‌اضافهٔ دیمنِ XPC
  <span dir="ltr">`com.apple.NetworkSharing`</span>؛ دیگر هیچ کار launchd با نام
  <span dir="ltr">`com.apple.InternetSharing`</span> وجود ندارد. bootpd کارِ DHCP را می‌کند؛ پروکسیِ
  mDNSResponder به DNS کلاینت‌ها جواب می‌دهد.

ساخته و آزمون واحد شده (با recorderها، بدون root):

- <span dir="ltr">`darwinnet_detect.go`</span>: <span dir="ltr">`ifconfig -a`</span>، <span dir="ltr">`route -n get default`</span>،
  <span dir="ltr">`networksetup -listallhardwareports`</span> و <span dir="ltr">`-getairportnetwork`</span>،
  خواندنِ endpoint و state و bypass domainهای SOCKS برای هر سرویسِ فعال با
  <span dir="ltr">`networksetup`</span>، سپس <span dir="ltr">`sysctl -e`</span> و <span dir="ltr">`pfctl -s info`</span>، به درونِ <span dir="ltr">`Facts`</span> مشترک.
- <span dir="ltr">`darwinnet_steps.go`</span>: یک anchor در pf با نام <span dir="ltr">`com.apple/250.CaspianBYOC`</span>
  (که <span dir="ltr">`anchor "com.apple/*"`</span> در <span dir="ltr">`/etc/pf.conf`</span> بدون ویرایشِ قواعد اصلی آن را
  ارزیابی می‌کند)، در یک تراکنش بار می‌شود. DNS کلاینت را به موتور هدایت می‌کند،
  ترافیک کلاینت را با <span dir="ltr">`route-to`</span> به درون utun می‌راند، هر چیز دیگری از کلاینت‌ها
  را در ورودِ <span dir="ltr">`bridge100`</span> مسدود می‌کند، و پنل را می‌گذارد بگذرد. مسیرِ سنجاق‌شده
  به سرور با <span dir="ltr">`route -n add -host`</span>. <span dir="ltr">`net.inet.ip.forwarding`</span> روشن. گامِ نشانیِ
  تونل نداریم: TUN داروینِ xray-core خودش 169.254.10.2/30 را می‌گذارد و اصرار
  دارد دستگاه <span dir="ltr">`utunN`</span> باشد، پس دستگاه <span dir="ltr">`utun100`</span> است.
- پروکسیِ موقت برای برنامه‌های میزبان: بعد از اینکه موتور
  <span dir="ltr">`127.0.0.1:10808`</span> را باز کرد، چهار گامِ ژورنال‌شدهٔ <span dir="ltr">`networksetup`</span> برای هر
  سرویسِ فعال، پروکسیِ قبلی را خاموش می‌کند، bypass domainهای محلی را ادغام
  می‌کند، endpoint بدون احراز هویت Caspian را می‌گذارد و آن را روشن می‌کند.
  teardown با ترتیب معکوس endpoint ازپیش‌تنظیم‌شده، bypass domainها و state
  اندازه‌گیری‌شده را برمی‌گرداند. سرویسی که endpoint نداشت خاموش می‌ماند؛
  <span dir="ltr">`networksetup`</span> فعلِ پاک‌کردن endpoint ندارد. پروکسیِ احرازشدهٔ موجود رد
  می‌شود، چون رمز پنهانش قابل بازیابی نیست.
- <span dir="ltr">`internetsharing.go`</span>: فایل تنظیمات را با کلیدهایی می‌نویسد که دامپ‌های واقعی
  و رشته‌های پلاگین نشان می‌دهند (NetworkName، NetworkPassword به شکل دادهٔ
  UTF-16LE، Channel، PrimaryService به عنوان UUID سرویسِ uplink،
  SharingDevices، SharingNetworkNumberStart/End/Mask از زیرشبکهٔ طرح)، آن را از
  راه <span dir="ltr">`scutil --prefs`</span> دوباره ذخیره می‌کند تا configd اعلان‌های commit و apply را
  بگیرد، اگر bridge ظاهر نشد یک بار دیمن را kickstart می‌کند، و bridge را
  بازمی‌خواند. شمار دستگاه‌ها از <span dir="ltr">`/var/db/dhcpd_leases`</span>.

UNVERIFIED تا وقتی با root روی یک مک اجرا شود، به این ترتیب (اسکریپت:
<span dir="ltr">`local/measure-internet-sharing.sh`</span>، در gitignore، پیش از ذخیره پاک‌سازی می‌کند):

1. اینکه پنجرهٔ Sharing در macOS 26 کلیدهای بالا را به همان شکل می‌نویسد، و رمزِ
   WPA را کجا نگه می‌دارد (در plist، یا در یک آیتمِ System keychain از راه API
   خصوصی HostAP در CoreWLAN). اگر keychain، درایور یک گامِ
   <span dir="ltr">`security add-generic-password`</span> با ویژگی‌های اندازه‌گیری‌شده نیاز دارد.
2. اینکه commit و apply در <span dir="ltr">`scutil --prefs`</span> روی 26.6 اشتراک را شروع می‌کند، یا
   <span dir="ltr">`launchctl kickstart -k system/com.apple.NetworkSharing`</span> می‌کند.
3. اینکه <span dir="ltr">`route-to`</span> در pf روی utun در pf اپل کار می‌کند (<span dir="ltr">`pf_route`</span> در xnu می‌گوید
   کار می‌کند؛ هیچ‌کس اجرایی منتشر نکرده)، و اینکه یک <span dir="ltr">`rdr`</span> در فرزندِ <span dir="ltr">`com.apple/*`</span>
   برای ترافیک <span dir="ltr">`bridge100`</span> رعایت می‌شود.
4. اینکه Internet Sharing با دیدنِ utun خودش را جمع نمی‌کند.
5. اینکه یک برنامهٔ عادیِ macOS که proxy-aware است (بدون پروکسیِ جداگانهٔ خود
   برنامه) هنگام اجرای Caspian آی‌پی خروجیِ تونل را نشان می‌دهد، و stop عادی،
   start ناموفق و بازیابی پس از توقف اجباریِ پروسه state قبلیِ همهٔ سرویس‌ها
   را برمی‌گردانند.

تنظیم SOCKS سیستم عمداً «موقت» نامیده شده است. برنامه‌هایی که تنظیمات پروکسی
macOS را نادیده می‌گیرند، UDP به‌طور کلی، و همهٔ DNS سیستم را پوشش نمی‌دهد.
تونل کامل میزبان، شامل DNS بدون fallback روی uplink فیزیکی، Option 1 در
<span dir="ltr">`backlog.md`</span> است و این پورت هنوز چنین ادعایی ندارد.

اجرای آن روی این مک یک uplink اترنت می‌خواهد (یک کابل در یکی از آداپتورهای
USB Ethernet)، <span dir="ltr">`sudo`</span>، و <span dir="ltr">`bash packaging/darwin/install-darwin.sh`</span>.

بسته‌های DMG روی macOS با برنامهٔ متن‌باز Go و ابزار سیستم <span dir="ltr">`hdiutil`</span>
ساخته می‌شوند. برای ساخت نسخهٔ Apple Silicon:

<div dir="ltr" align="left">

```sh
bash packaging/darwin/build-dmg.sh v0.2.4 arm64
```

</div>

در تصویر دیسک، Caspian.app، باینری برنامه، دو فایل launchd، میان‌بر Applications
و راهنمای نصب قرار دارند. هنگام اجرا، برنامهٔ کنترل باینری داخل خودش را بایت‌به‌بایت
با <span dir="ltr">`/usr/local/bin/caspian`</span> مقایسه می‌کند. اگر باینری نصب نشده یا متفاوت باشد،
فرایند نصب با مجوز مدیر یک بار خودکار آغاز می‌شود؛ اگر یکسان باشد، بدون پرسش
مستقیم به حالت آماده می‌رود. بنابراین پنل قدیمی اما در دسترس نمی‌تواند نیاز به
به‌روزرسانی را پنهان کند. نسخه‌های Intel و Apple Silicon جداگانه ساخته می‌شوند.

پنجرهٔ کنترل ابتدا متن انگلیسی و سپس ترجمهٔ فارسی را دقیقاً از همان لبهٔ چپ
نشان می‌دهد؛ ترتیب خواندن انگلیسی چپ‌به‌راست و فارسی راست‌به‌چپ باقی می‌ماند.
اندازهٔ متن وضعیت ۲۲ پوینت، متن
توضیح ۱۶ تا ۱۷ پوینت و همهٔ هدف‌های عملیاتی بزرگ و دوزبانه‌اند. علامت مبهم
چیدمان از شبکهٔ ۸ پوینتی استفاده می‌کند: حاشیهٔ پنجره ۳۲ پوینت، فاصلهٔ بخش‌ها
۲۴ پوینت، فاصلهٔ ردیف‌ها ۱۶ پوینت، فاصلهٔ داخلی کارت ۲۰ پوینت، ستون عملیات
۲۴۸ پوینت و ارتفاع دکمه‌ها ۷۲ پوینت است. خروجی متغیر و گزینه‌های پیشرفته در
یک پنجرهٔ ثابت پیمایش می‌شوند و دیگر اندازهٔ پنجره را تغییر نمی‌دهند.
بازشدن با دکمهٔ صریح **Advanced options / گزینه‌های پیشرفته** جایگزین شده است.
پابرگ سه ستون هم‌اندازه دارد: اطلاعات ثابت نسخه، پیوند دوزبانهٔ GitHub با
محدودهٔ کلیک فقط روی متن نمایان، و نام دوزبانهٔ
`Iman Samizadeh / ایمان سمیع زاده`. ساخت برچسب‌خوردهٔ GitHub
Actions اگر نسخهٔ نمایشی با برچسب انتشار یکسان نباشد متوقف می‌شود؛ ساخت محلی
نیز صادقانه به‌عنوان پیش‌نمایش بر پایهٔ آن انتشار نشان داده می‌شود.

گزینهٔ Set up Caspian یا Update Caspian از پنجرهٔ مجوز مدیر macOS استفاده می‌کند. در نصب
اول، رمز صفحه نمایش داده می‌شود و دکمهٔ Copy آن را کپی می‌کند. به‌روزرسانی
رمز فعلی را حفظ می‌کند.

اگر رمز گم شد، در Caspian.app گزینهٔ Reset Password را انتخاب کنید. پس از
احراز هویت مدیر، صفحه متوقف می‌شود، فقط رمز آن تغییر می‌کند و دوباره اجرا
می‌شود. رمز تازه پیش از باز شدن صفحه نمایش داده می‌شود. کاربران واردشده
می‌توانند در داشبورد با وارد کردن رمز فعلی و دو بار رمز تازه، آن را تغییر
دهند. پس از تغییر موفق، همهٔ نشست‌های قبلی خارج می‌شوند.

## Windows

در 2026-09-03 با مالک تصمیم گرفته شد: Mobile Hotspot نقطهٔ دسترسی است (با
کمک‌برنامهٔ C# در <span dir="ltr">`tools/caspian-tethering`</span>)، کلِ میزبان تونل می‌شود چون Windows
مسیریابیِ بر پایهٔ مبدأ ندارد، و fail-closed با فیلترهای Windows Filtering
Platform در لایهٔ forwarding اعمال می‌شود، چون قواعد فایروال معمولی هرگز ترافیک
forward شده را نمی‌بینند.

اینجا ساخته و برای <span dir="ltr">`windows/amd64`</span> و <span dir="ltr">`windows/arm64`</span> کامپایل‌چک شده؛ کمک‌برنامهٔ
C# روی این مک در برابر projection واقعی WinRT کامپایل می‌شود:

- <span dir="ltr">`winnet.go`</span>: بک‌اند. شبه‌باینری‌های <span dir="ltr">`iphlpapi`</span>، <span dir="ltr">`wfp`</span>، <span dir="ltr">`wintun`</span>. پیش از موتور:
  ساختِ آداپتور تونل (تا فیلترها بتوانند نامش را ببرند)، بارگذاری فیلترها،
  forwarding روشن، مسیرِ سنجاق‌شدهٔ میزبان به سرور. پس از موتور: نشانی، متریکِ رابط
  صفر، مسیرِ پیش‌فرض از تونل. زیرشبکهٔ هات‌اسپات به 192.168.137.0/24 سنجاق شده،
  که Internet Connection Sharing سرو می‌کند و نمی‌شود جور دیگری به آن گفت.
- <span dir="ltr">`exec_windows.go`</span> و <span dir="ltr">`winsys_windows.go`</span>: اجراکننده. ساختارها فیلد به فیلد از
  بسته‌های MIT <span dir="ltr">`winipcfg`</span> و <span dir="ltr">`firewall`</span> در WireGuard کپی شده‌اند؛ GUIDهای لایه و
  شرطِ WFP از هدرهای Microsoft به واسطهٔ tailscale/wf. فیلترها ماندگارند (از
  پروسه بیشتر عمر می‌کنند، پس یک crash جعبه را بسته می‌گذارد) زیر یک provider و
  sublayer با کلیدهای ثابت.
- <span dir="ltr">`mobilehotspot.go`</span> و کمک‌برنامه: یک پروسه برای هر عمل، JSON در ورودی، یک خط
  JSON در خروجی. کمک‌برنامه توانِ tethering را می‌سنجد و دلیلِ Windows را با نام
  می‌گزارد، SSID و رمز و باند را اعمال می‌کند، مهلتِ پنج‌دقیقه‌ایِ بی‌کلاینت را
  خاموش می‌کند، شروع می‌کند، و حالت را بازمی‌خواند.
- <span dir="ltr">`transport_windows.go`</span>: named pipe در <span dir="ltr">`\\.\pipe\caspian-priv`</span>، توصیف‌گر امنیتی
  که SYSTEM، Administrators و حساب سرویسِ مجازیِ پنل را می‌پذیرد؛ SID کلاینت از
  راه impersonation خوانده می‌شود.
- <span dir="ltr">`packaging/windows/install.ps1`</span>: دو سرویس، ACLها، رمزِ نخستین اجرا.

اندازه‌گیری روی یک دستگاه واقعی Windows در 2026-09-03 نشان داد که <span dir="ltr">`readInventory`</span>
در <span dir="ltr">`exec_windows.go`</span> همهٔ سطرهای <span dir="ltr">`GetIfTable2Ex`</span> را می‌خواند. درایور NDIS برای هر
رادیوی واقعی چند رابط فیلتر مجازی هم گزارش می‌کند. در نتیجه <span dir="ltr">`windowsDetect`</span> هر
رابط مجازی را یک رادیوی جدا حساب می‌کرد و برنامه یکی از آن‌ها را برای هات‌اسپات
انتخاب می‌کرد. بررسی وجود برنامه نیز ایراد مشابهی داشت: <span dir="ltr">`findProgram`</span> بیت اجرای
Unix را لازم می‌دانست، اما <span dir="ltr">`os.Stat`</span> این بیت را در Windows تنظیم نمی‌کند.

هر دو ایراد رفع شده‌اند. اکنون <span dir="ltr">`readInventory`</span> سطری را که پرچم
<span dir="ltr">`FilterInterface`</span> در <span dir="ltr">`InterfaceAndOperStatusFlags`</span> دارد کنار می‌گذارد. همچنین
<span dir="ltr">`findProgram`</span> فقط در سیستم‌عامل‌هایی که بیت اجرا دارند آن را بررسی می‌کند. پس
<span dir="ltr">`caspian.exe check`</span> رابط Ethernet واقعی را برای ورودی و رادیوی Wi-Fi واقعی را
برای هات‌اسپات انتخاب می‌کند.

برای تمام کردن روی ماشین Windows، به این ترتیب:

1. ساخت: <span dir="ltr">`go build -o caspian.exe ./cmd/caspian`</span> و
   <span dir="ltr">`dotnet publish -c Release -r win-x64 -o out tools/caspian-tethering`</span>؛
   <span dir="ltr">`wintun.dll`</span> را از wintun.net بگیرید (نامش را تغییر ندهید).
2. <span dir="ltr">`caspian.exe check`</span> از یک پرامپتِ elevated: فهرست آداپتورها، طرح.
3. کمک‌برنامه با دست: `echo {"op":"status","uplink":"Ethernet"} |
   caspian-tethering.exe status`، سپس یک start با نام‌بردنِ دانگل به عنوان
   آداپتور. این تعیین می‌کند که overload <span dir="ltr">`(profile, NetworkAdapter)`</span> دانگل را
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

- xray-core از 2026-09-03 نسخهٔ v26.4.15 است. <span dir="ltr">`Handler.Close`</span> آن موتور را وادار
  می‌کند دستگاه TUN را روی هر پلتفرمی آزاد کند؛ مسیرِ آزادسازیِ فقط‌لینوکسی در
  <span dir="ltr">`internal/engine/tundevice_linux.go`</span> به عنوان شبکهٔ ایمنیِ اندازه‌گیری‌شده نگه
  داشته شده تا وقتی روی دستگاه دوباره اندازه گرفته شود که زائد است.
- <span dir="ltr">`docs/LAYOUT.md`</span> تنها جدولِ لینوکس را مستند می‌کند. <span dir="ltr">`cmd/caspian/paths_*.go`</span>
  جدول‌های macOS و Windows را دارند؛ سند باید هر دو را بگیرد.
- <span dir="ltr">`install.sh`</span> همچنان هر چیزی که لینوکس نیست را رد می‌کند، به قصد؛ نصاب‌های
  پلتفرمی زیر <span dir="ltr">`packaging/darwin`</span> و <span dir="ltr">`packaging/windows`</span> هستند.
- روی Windows حفاظتِ 0600 که <span dir="ltr">`internal/state`</span> قول می‌دهد در کار نیست
  (<span dir="ltr">`perm_other.go`</span> این را می‌گوید)؛ ACLهای نصاب جای آن را می‌گیرند.

</div>
