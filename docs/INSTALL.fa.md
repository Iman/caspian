<div dir="rtl" align="right">

# نصب کاسپین (Caspian-BYOC)

🇮🇷 **فارسی** | [🇬🇧 English](INSTALL.md) | [🇷🇺 Русский](../README.ru.md) | [🇨🇳 中文](../README.zh.md)

> نسخهٔ انگلیسی: [<span dir="ltr">`docs/INSTALL.md`</span>](INSTALL.md). آزمون‌ها نسخهٔ انگلیسی را
> می‌خوانند و مرجع همان است. اگر این دو با هم اختلاف داشتند، انگلیسی درست است.

یک فرمان، و بعد پنل.

<div dir="ltr" align="left">

    sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"


</div>

مالک مخزن <span dir="ltr">`Iman`</span> است، هم در <span dir="ltr">`docs/LAYOUT.md`</span> و هم در <span dir="ltr">`install.sh`</span> که مقدار
پیش‌فرض <span dir="ltr">`CASPIAN_ORG`</span> همان است. آرتیفکت‌ها و فایل <span dir="ltr">`SHA256SUMS`</span> را
<span dir="ltr">`.github/workflows/release.yml`</span> هنگام push شدن یک تگ نسخه می‌سازد و منتشر
می‌کند.

پس از پایان کارِ نصب‌کننده، دو چیز چاپ می‌شود، نشانی پنل و یک رمز عبور نخستین
اجرا، و هیچ چیز مهم دیگری. هر کار بعدی در پنل انجام می‌شود.

## چه می‌کند، به ترتیب

1. پیش از دست‌زدن به ماشین، هر چیزی را که نمی‌تواند رویش نصب شود رد می‌کند: اگر
   Linux نباشد، اگر معماری آرتیفکتی نداشته باشد، اگر systemd نباشد، یا اگر
   systemd قدیمی‌تر از 240 باشد. هر رد‌کردن، آنچه را یافته و آنچه را پشتیبانی
   می‌شود نام می‌برد و با کد غیرصفر خارج می‌شود.
2. درمی‌آورد که این یک نصب تازه است یا یک ارتقا، با نگاه‌کردن به
   <span dir="ltr">`/usr/local/bin/caspian`</span>.
3. مدیر بسته را پیدا می‌کند و فقط وابستگی‌های غایب را نصب می‌کند. اول آنها را
   فهرست می‌کند، و اگر ترمینالی برای پرسیدن باشد پیش از نصب می‌پرسد.
4. آرتیفکت انتشار و فایل چک‌سام را دانلود می‌کند، SHA-256 را راستی‌آزمایی می‌کند،
   و در برابر هر ناهم‌خوانی سر باز می‌زند. این کار پیش از آن رخ می‌دهد که چیزی
   روی دستگاه متوقف یا جایگزین شود، پس یک دانلود ناموفق نصبِ کارکن را دقیقاً به
   همان شکل که بود رها می‌کند.
5. اگر <span dir="ltr">`caspian-panel.service`</span> و بعد <span dir="ltr">`caspian.service`</span> آنجا باشند، متوقفشان
   می‌کند.
6. گروه و کاربر سیستمی <span dir="ltr">`caspian`</span> را می‌سازد.
7. دایرکتوری‌های <span dir="ltr">`/var/lib/caspian`</span>، <span dir="ltr">`/run/caspian`</span> و <span dir="ltr">`/run/caspian/dnsmasq`</span> را
   با مجوزهای <span dir="ltr">`docs/LAYOUT.md`</span> می‌سازد. هرگز به محتوای <span dir="ltr">`/var/lib/caspian`</span> دست
   نمی‌زند.
8. باینری را در <span dir="ltr">`/usr/local/bin/caspian`</span> نصب می‌کند.
9. دو یونیت systemd، قطعهٔ tmpfiles و قطعهٔ modules-load را می‌نویسد، سپس systemd
   را دوباره بارگذاری می‌کند، <span dir="ltr">`/run/caspian`</span> را می‌سازد و ماژول‌های <span dir="ltr">`tun`</span> و
   <span dir="ltr">`nf_tables`</span> را بارگذاری می‌کند.
10. فقط در نصب تازه، یک رمز عبور نخستین اجرا تولید می‌کند و آن را جایی می‌گذارد
    که پنل برش دارد.
11. هر دو یونیت را فعال و شروع می‌کند.
12. اگر بتواند، حذف‌کننده را در <span dir="ltr">`/usr/local/bin/caspian-uninstall`</span> کپی می‌کند.
13. نشانی پنل و رمز عبور را چاپ می‌کند.

## پیش‌نیازها، و آنچه رد می‌کند

| پیش‌نیاز | پیام رد‌کردن چه چیزی را نام می‌برد |
|---|---|
| Linux | آنچه <span dir="ltr">`uname -s`</span> گفت |
| x86_64، aarch64، armv7l یا armv6l | آنچه <span dir="ltr">`uname -m`</span> گفت |
| systemd، نسخهٔ 240 یا جدیدتر | محتوای <span dir="ltr">`/proc/1/comm`</span>، یا نسخه‌ای که یافت شد |
| root | اینکه به‌جایش چه اجرا کند |

systemd نسخهٔ 240 همان نسخه‌ای است که <span dir="ltr">`Type=exec`</span> را معرفی کرد، و هر دو یونیت از
آن استفاده می‌کنند.

<span dir="ltr">`armv8l`</span> عمداً نگاشت نشده است. آن یک فضای کاربریِ 32 بیتی روی کرنل 64 بیتی است،
<span dir="ltr">`docs/LAYOUT.md`</span> نمی‌گوید کدام آرتیفکت را باید بگیرد، و حدس‌زدن همان کاری است که
باگ armv6 پایین از آن پدید آمد. پس رد می‌کند و همین را می‌گوید.

## نگاشت معماری، و الزامی که در سمت انتشار همراهش می‌آید

آرتیفکت‌های انتشار از نام‌های Go پیروی می‌کنند، نه از نام‌های کرنل:

| <span dir="ltr">`uname -m`</span> | آرتیفکت |
|---|---|
| <span dir="ltr">`x86_64`</span> | <span dir="ltr">`caspian-linux-amd64`</span> |
| <span dir="ltr">`aarch64`</span> | <span dir="ltr">`caspian-linux-arm64`</span> |
| <span dir="ltr">`armv7l`</span> | <span dir="ltr">`caspian-linux-arm`</span> |
| <span dir="ltr">`armv6l`</span> | <span dir="ltr">`caspian-linux-arm`</span> |

پروژه‌ای پیشین در همین کارگاه armv6 را روی آرتیفکت armv7 نگاشت کرد و مدل‌های
قدیمی‌تر Pi را خراب کرد. یک ساخت armv7 از دستورهایی استفاده می‌کند که ARM1176
داخل Pi 1، Pi Zero یا Pi Zero W آنها را ندارد، پس باینری تمیز نصب می‌شود و بعد
در اولین اجرا با یک illegal instruction می‌میرد.

هر دو مقدار 32 بیتی اینجا روی یک آرتیفکت نگاشت می‌شوند، که همان چیزی است که
<span dir="ltr">`docs/LAYOUT.md`</span> تثبیت می‌کند. این فقط تا وقتی درست است که <span dir="ltr">`caspian-linux-arm`</span>
با <span dir="ltr">`GOARM=6`</span> ساخته شود.

**ساختن آن با <span dir="ltr">`GOARM=7`</span> همان باگ را یک لایه بالاتر برمی‌گرداند، داخل خط لولهٔ
انتشار به‌جای داخل نصب‌کننده، جایی که هیچ آزمونی در این مخزن نمی‌تواند ببیندش.**
ساختِ انتشار باید برای آرتیفکت <span dir="ltr">`linux/arm`</span> مقدار <span dir="ltr">`GOARM=6`</span> را تنظیم کند.

## وابستگی‌ها

از <span dir="ltr">`docs/LAYOUT.md`</span>: <span dir="ltr">`hostapd`</span>، <span dir="ltr">`dnsmasq`</span>، <span dir="ltr">`nftables`</span>، <span dir="ltr">`iw`</span>، <span dir="ltr">`iproute2`</span>.

نصب‌کننده به‌جای پرسیدن از پایگاه دادهٔ بسته‌ها، وجود فرمانی را که هر بسته فراهم
می‌کند آزمایش می‌کند، چون «آیا <span dir="ltr">`nft`</span> روی این دستگاه هست» همه‌جا یک پاسخ دارد:

| بسته | فرمانی که آزمایش می‌شود |
|---|---|
| hostapd | <span dir="ltr">`hostapd`</span> |
| dnsmasq | <span dir="ltr">`dnsmasq`</span> |
| nftables | <span dir="ltr">`nft`</span> |
| iw | <span dir="ltr">`iw`</span> |
| iproute2 | <span dir="ltr">`ip`</span> |

مدیرهای بسته به این ترتیب تشخیص داده می‌شوند: <span dir="ltr">`apt-get`</span>، <span dir="ltr">`dnf`</span>، <span dir="ltr">`yum`</span>، <span dir="ltr">`pacman`</span>،
<span dir="ltr">`zypper`</span>، <span dir="ltr">`apk`</span>. یک نام بسته میان آنها فرق دارد: روی <span dir="ltr">`dnf`</span> و <span dir="ltr">`yum`</span> نام
<span dir="ltr">`iproute2`</span> برابر <span dir="ltr">`iproute`</span> است.

اگر نام یک بسته روی توزیعی غلط از آب درآید، شکست به‌صورت پیامی است که فرمانِ هنوز
غایب و بسته‌ای که امتحان شد را نام می‌برد، و پس از آن ادامه‌ندادن. هرگز یک
نیمه‌نصبِ بی‌صدا نیست.

## دانلود، و اینکه چک‌سام چه چیزی را اثبات می‌کند و چه چیزی را نه

آرتیفکت و یک فایل <span dir="ltr">`SHA256SUMS`</span> از یک دایرکتوری انتشار واکشی می‌شوند. آن فایل
قالب <span dir="ltr">`sha256sum`</span> دارد، یک سطر برای هر آرتیفکت:

<div dir="ltr" align="left">

    <64 hex characters>  caspian-linux-arm64


</div>

نصب‌کننده چهار چیز را با چهار پیام متفاوت رد می‌کند، چون چهار مسئلهٔ متفاوت‌اند:
نبودن مدخلی برای این آرتیفکت، مدخلی که هش SHA-256 نیست، هشی که مطابقت نمی‌کند، و
آدرسی که HTTPS نیست.

آنچه این بررسی اثبات می‌کند: آرتیفکت همانی است که فایل چک‌سام توصیفش می‌کند. یک
دانلود بریده، یک آینهٔ خراب و یک نسخهٔ کهنه روی CDN را می‌گیرد.

آنچه اثبات نمی‌کند: هر دو فایل از یک جا می‌آیند، پس کسی که آن جا را در دست دارد
می‌تواند یک جفتِ هماهنگ سرو کند. HTTPS به میزبان انتشار همان چیزی است که در برابر
این دفاع می‌کند، و به همین دلیل یک آدرس پایهٔ متن‌ساده رد می‌شود، مگر آنکه
<span dir="ltr">`CASPIAN_ALLOW_INSECURE_URL=1`</span> برای آزمایش محلی تنظیم شده باشد.

## دوبار اجرا کردنش

اجرای دوم یک ارتقا است. سرویس‌ها را متوقف می‌کند، باینری را جایگزین می‌کند،
یونیت‌ها را دوباره می‌نویسد، و دوباره راه می‌اندازد. به <span dir="ltr">`/var/lib/caspian`</span> دست
نمی‌زند، پس کانفیگ پروکسی، نام و رمز هات‌اسپات، و رمز پنل همه باقی می‌مانند. رمز
تازه‌ای تولید نمی‌کند و می‌گوید <span dir="ltr">`Password: unchanged from the previous install`</span>.

## چه چیزهایی ساخته می‌شود

| مسیر | مجوز | مالک | چیست |
|---|---|---|---|
| <span dir="ltr">`/usr/local/bin/caspian`</span> | 0755 | root | تنها باینری |
| <span dir="ltr">`/usr/local/bin/caspian-uninstall`</span> | 0755 | root | یک نسخهٔ محلی از <span dir="ltr">`uninstall.sh`</span> |
| <span dir="ltr">`/var/lib/caspian`</span> | 0700 | caspian | وضعیت. در ارتقا هرگز حذف نمی‌شود |
| <span dir="ltr">`/run/caspian`</span> | 0750 | root:caspian | زمان اجرا، در هر بوت دوباره ساخته می‌شود |
| <span dir="ltr">`/run/caspian/dnsmasq`</span> | 0700 | caspian | فایل pid مربوط به dnsmasq، در هر بوت دوباره ساخته می‌شود |
| <span dir="ltr">`/etc/systemd/system/caspian.service`</span> | 0644 | root | یونیت ممتاز |
| <span dir="ltr">`/etc/systemd/system/caspian-panel.service`</span> | 0644 | root | یونیت پنل |
| <span dir="ltr">`/etc/tmpfiles.d/caspian.conf`</span> | 0644 | root | هر دو دایرکتوری <span dir="ltr">`/run`</span> را در بوت دوباره می‌سازد |
| <span dir="ltr">`/etc/modules-load.d/caspian.conf`</span> | 0644 | root | ماژول‌های <span dir="ltr">`tun`</span> و <span dir="ltr">`nf_tables`</span> را در بوت بارگذاری می‌کند |

هیچ <span dir="ltr">`/etc/caspian`</span>‌ای وجود ندارد. تا 2026-08-30 در چیدمان بود و حذف شد، چون
فایل‌های تولیدشدهٔ hostapd و dnsmasq که قرار بود در آن باشند زیر <span dir="ltr">`/run`</span> هستند:
در هر بار شروع دوباره نوشته می‌شوند، عبارت عبور WPA2 را حمل می‌کنند، و <span dir="ltr">`/run`</span>
یک tmpfs است، پس یک اعتبارنامه در فایلی که کسی از وجودش خبر ندارد باقی نمی‌ماند.

tmpfs بودنِ <span dir="ltr">`/run`</span> همچنین دلیل این است که هیچ‌کدام از دو دایرکتوریِ زمان اجرا را
نمی‌شود یک بار با نصب‌کننده ساخت و انتظار داشت از یک راه‌اندازی دوباره جان سالم به
در ببرند. قطعهٔ tmpfiles هر دو را دوباره می‌سازد، دقیقاً با همان مجوزها و
مالکیت‌هایی که چیدمان تثبیت می‌کند.

سه مسیر دیگر در زمان اجرا پدیدار می‌شوند و ساختنشان کارِ نصب‌کننده نیست:
<span dir="ltr">`/run/caspian/hostapd.conf`</span> و <span dir="ltr">`/run/caspian/dnsmasq.conf`</span> (مجوز 0600، مالک root،
در هر بار شروع دوباره نوشته می‌شوند) و <span dir="ltr">`/run/caspian/dnsmasq/dnsmasq.pid`</span> (مجوز
0644، مالک caspian، خودِ dnsmasq آن را می‌نویسد).

### چرا dnsmasq دایرکتوری خودش را دارد، و تله‌ای که کنارش هست

dnsmasq به حساب <span dir="ltr">`caspian`</span> تنزل می‌کند و بعد فایل pid خود را می‌نویسد.
<span dir="ltr">`/run/caspian`</span> با مجوز 0750 و مالکیت root:caspian است، پس گروه می‌تواند فهرست آن
را ببیند و نمی‌تواند در آن بنویسد، و اینکه dnsmasq فایل pid را پیش از تنزل امتیاز
می‌نویسد یا پس از آن، خاصیتی از dnsmasq است که هیچ‌کس اینجا آن را اندازه نگرفته
است. دادنِ دایرکتوری‌ای به dnsmasq که مالکش خودش باشد یعنی پاسخ این پرسش دیگر
اهمیتی ندارد، و این بهتر از اندازه‌گرفتن یک‌بارهٔ آن و بعد وابسته‌شدن به درست‌ماندنش
پس از یک ارتقای dnsmasq است.

**فایل pid‌ای را که نوشته نمی‌شود با نوشتنی‌کردنِ <span dir="ltr">`/run/caspian`</span> برای گروه درست
نکنید.** اجازهٔ ساخت و حذف در یک دایرکتوری از خودِ دایرکتوری می‌آید، نه از فایل.
پس اگر <span dir="ltr">`/run/caspian`</span> برای گروه نوشتنی باشد، حسابِ بدون‌امتیازِ پنل می‌تواند
<span dir="ltr">`hostapd.conf`</span> را حذف کند و فایل خودش را بنویسد، و سپس سمت ممتاز همان فایل را به
hostapd که با root اجرا می‌شود تحویل می‌دهد. این کار یک دردسر کوچکِ فایل pid را
به ارتقای امتیاز محلی تبدیل می‌کند. همین هشدار در <span dir="ltr">`docs/LAYOUT.md`</span> هست، در
<span dir="ltr">`install.sh`</span> کنار سطری که دایرکتوری را می‌سازد، و در
<span dir="ltr">`packaging/caspian.tmpfiles.conf`</span> کنار سطری که آن را دوباره می‌سازد، چون آدم وقتی
به این تله برمی‌خورد همان‌جا ایستاده است.

## پورت‌ها

نصب‌کننده هیچ پورتی را تنظیم نمی‌کند و هیچ‌کدام را بررسی نمی‌کند. یکی را چاپ
می‌کند، پورت پنل را، و آن را به‌جای حافظه از جدول «پورت‌ها» در <span dir="ltr">`docs/LAYOUT.md`</span>
می‌گیرد:

| پورت | اتصال به | چیست | با چه چیزی هم‌خوان است |
|---|---|---|---|
| 53 | رابط هات‌اسپات | dnsmasq، یعنی DHCP و DNS برای دستگاه‌های وصل‌شده | <span dir="ltr">`internal/netcfg/plan.go`</span>، <span dir="ltr">`DNSPort`</span> |
| 5354 | 127.0.0.1 | شنوندهٔ محلی DNS موتور | <span dir="ltr">`internal/xcfg`</span>، <span dir="ltr">`DefaultLocalDNSPort`</span> |
| 8088 | نشانی پنل | پنل وب | <span dir="ltr">`internal/netcfg/plan.go`</span>، <span dir="ltr">`PanelPort`</span> |
| 10808 | 127.0.0.1 | SOCKS، برای عیب‌یابی، اثبات IP خروجی و پروکسی موقت سیستم در macOS | <span dir="ltr">`internal/xcfg`</span>، <span dir="ltr">`DefaultSocksPort`</span> |

آنکه بی‌صدا خراب می‌شود 5354 است: dnsmasq فقط به آنجا فوروارد می‌کند و موتور
آنجا گوش می‌دهد، و اگر این دو از هم دور شوند، DNS برای هر دستگاهِ وصل‌شده از کار
می‌افتد در حالی که هات‌اسپات و تونل هر دو همچنان سالم به نظر می‌رسند. هیچ‌یک از دو
سرِ این جفت را نصب‌کننده تنظیم نمی‌کند، و آزمون مقابله‌ای برای آن جایش در
<span dir="ltr">`internal/xcfg`</span> است، جایی که آن آزمون هم‌اکنون هست.

## دو یونیت

<span dir="ltr">`caspian.service`</span> با root اجرا می‌شود و مالک مسیرها، فایروال، اکسس‌پوینت و موتور
است. <span dir="ltr">`caspian-panel.service`</span> با کاربر <span dir="ltr">`caspian`</span> اجرا می‌شود و مالک رابط وب است و
هیچ چیز ممتازی. پنل با <span dir="ltr">`Wants=`</span> پس از سرویس ممتاز مرتب می‌شود و هرگز با
<span dir="ltr">`Requires=`</span>. بخش 5.6 طراحی ثبت می‌کند که کاربری که نتواند به پنل برسد نمی‌تواند
هیچ چیز را درست کند، پس پنل باید بالا بیاید و بگوید چه ایرادی هست، حتی وقتی سمت
ممتاز در شروع شکست خورده باشد.

فایل‌های یونیت در <span dir="ltr">`packaging/`</span> منبع حقیقت‌اند. <span dir="ltr">`install.sh`</span> یک نسخهٔ بایت‌به‌بایت
یکسان از هر کدام را درون خود دارد، چون اسکریپتی که از <span dir="ltr">`curl`</span> لوله می‌شود مخزنی
برای خواندن ندارد، و چون دانلودکردنِ آنها آرتیفکت‌های راستی‌آزمایی‌نشده‌ای را کنار
همان یکی می‌گذاشت که نصب‌کننده زحمت چک‌سام‌کردنش را می‌کشد.
<span dir="ltr">`packaging/test-install.sh`</span> اگر یکی از این نسخه‌ها واگرا شود شکست می‌خورد.

هر دو یونیت سخت‌سازی شده‌اند، و هر دستور در آنها می‌گوید در برابر چه چیزی محافظت
می‌کند. چهار دستور در یونیت ممتاز عمداً **خاموش** هستند، و هر کدام می‌گوید چه چیزی
را خراب می‌کرد:

| تنظیم‌نشده | چه چیزی را خراب می‌کرد |
|---|---|
| <span dir="ltr">`PrivateDevices=yes`</span> | <span dir="ltr">`/dev/net/tun`</span> را برمی‌دارد، پس موتور نمی‌تواند تونل را بسازد. با <span dir="ltr">`DevicePolicy=closed`</span> به‌علاوهٔ <span dir="ltr">`DeviceAllow`</span> برای <span dir="ltr">`/dev/net/tun`</span> و <span dir="ltr">`/dev/rfkill`</span> جایگزین شده است |
| <span dir="ltr">`ProtectKernelTunables=yes`</span> | <span dir="ltr">`/proc/sys`</span> را فقط‌خواندنی mount می‌کند، پس <span dir="ltr">`rp_filter`</span> تنظیم نمی‌شود. بخش 4.2 طراحی: یک <span dir="ltr">`rp_filter`</span> غلط تونلی می‌دهد که وصل می‌شود و چیزی حمل نمی‌کند |
| <span dir="ltr">`ProtectKernelModules=yes`</span> | بارگذاری بر حسب نیازِ ماژول‌ها را که مجموعه‌قواعد nftables برمی‌انگیزد ممنوع می‌کند |
| <span dir="ltr">`ProtectProc=invisible`</span> | فرایندهای دیگر را از <span dir="ltr">`pgrep`</span> پنهان می‌کند، پس ناظر هرگز نمی‌تواند یک <span dir="ltr">`hostapd`</span> یا <span dir="ltr">`dnsmasq`</span> سرگردان را که رادیو را در دست دارد پیدا کند |

یونیت پنل هیچ‌یک از این قیدها را ندارد، پس تا جایی که systemd اجازه می‌دهد تنگ
است: هیچ capability‌ای در هیچ‌یک از دو مجموعه، به‌علاوهٔ <span dir="ltr">`PrivateDevices=yes`</span>،
<span dir="ltr">`ProtectProc=invisible`</span>، <span dir="ltr">`ProcSubset=pid`</span>، <span dir="ltr">`MemoryDenyWriteExecute=yes`</span>، و بدون
<span dir="ltr">`AF_NETLINK`</span>.

## نخستین اجرا

نصب‌کننده این را چاپ می‌کند:

<div dir="ltr" align="left">

    Panel:    http://192.168.4.31:8088/
    Password: nvbqd-3kx7m-rjhta-92wpe


</div>

آن نشانی و آن رمز، کل تحویل کار است. هر چه پس از آنها می‌آید روی همین صفحه رخ
می‌دهد:

![پنل، با تونل بالا و پیش از آنکه دستگاهی وصل شود](images/panel-fa.png)

پورت 8088 است، از «پورت‌ها» در <span dir="ltr">`docs/LAYOUT.md`</span>. بخش پورت‌های بالا را ببینید.

نشانی، نشانی IPv4 سراسریِ کنونی دستگاه است. این یک تلاش با بهترین کوشش است و
دانستن دلیلش می‌ارزد: بخش 5.6 طراحی می‌گوید پنل روی رابط هات‌اسپات گوش می‌دهد، و
هات‌اسپات تا وقتی کاربر روشنش نکند وجود ندارد، پس در پایان یک نصب تنها نشانی
موجود همان است که دستگاه از پیش داشت. بخش 5.6 این را به‌عنوان خطری ثبت می‌کند که
v1 هنوز باید پاسخش را بدهد.

رمز عبور بیست کاراکتر از یک الفبای سی‌ودو کاراکتری است، که صد بیت می‌شود. الفبا
<span dir="ltr">`0`</span>، <span dir="ltr">`O`</span>، <span dir="ltr">`1`</span>، <span dir="ltr">`l`</span> و <span dir="ltr">`I`</span> را کنار می‌گذارد، چون کسی که این رمز را انتخاب نکرده
است باید آن را از روی ترمینال بخواند و در تلفن تایپ کند.

### تحویل، که به همکاری پنل نیاز دارد

نصب‌کننده متن ساده را در <span dir="ltr">`/var/lib/caspian/first-run-password`</span> می‌نویسد، با مجوز
0600، مالکیت <span dir="ltr">`caspian`</span>، داخل یک دایرکتوری 0700. قرارداد این است:

> در نخستین شروع، پنل آن فایل را می‌خواند، محتوایش را به
> <span dir="ltr">`state.Store.SetPanelPassword`</span> می‌دهد که آن را با argon2id هش می‌کند، و بعد
> فایل را حذف می‌کند.

یک فایل، و نه یک آرگومان خط فرمان یا یک متغیر محیطی، چون هر دوی آنها از <span dir="ltr">`/proc`</span>
برای هر چیزی روی دستگاه خواندنی‌اند.

**این نیمه پیاده‌سازی شده است.** <span dir="ltr">`cmd/caspian/firstrun.go`</span> تابع
<span dir="ltr">`consumeFirstRunPassword`</span> را فراهم می‌کند، <span dir="ltr">`cmd/caspian/serve_panel.go`</span> آن را در
شروع پنل صدا می‌زند، و <span dir="ltr">`cmd/caspian/firstrun_test.go`</span> پوششش می‌دهد. رمزی که چاپ
می‌شود کار می‌کند.

آن بند قبلاً خلاف این را می‌گفت: اینکه <span dir="ltr">`cmd/`</span> خالی است، هیچ چیز آن فایل را مصرف
نمی‌کند، و رمز چاپ‌شده کار نمی‌کند. آن متن پیش از آن نوشته شده بود که <span dir="ltr">`019fba6`</span>
در 2026-08-30 <span dir="ltr">`cmd/caspian`</span> را پر کند، و کسی برنگشت سراغش. در 2026-08-31 اصلاح
شد. دو سند دیگر آن تغییر را همان روز ثبت کردند و این یکی نکرد، که راه معمولی کهنه
شدن یک سند است: کسی که تغییر را می‌دهد فایلی را به‌روز می‌کند که جلوی چشمش است.
دو چیز که از پیش در درخت بودند نیمی از راه را آمده بودند:
<span dir="ltr">`state.Store.SetPanelPassword`</span> وجود دارد، و <span dir="ltr">`state.ErrNoPanelPassword`</span> به‌عنوان
سیگنالِ نمایشِ صفحهٔ راه‌اندازی توسط پنل مستند شده است، که پاسخ درست برای وقتی است
که آن فایل غایب باشد.

نصب‌کننده هرگز کانفیگ پروکسی کاربر را نمی‌خواند، چاپ نمی‌کند و ثبت نمی‌کند. دلیلی
هم برای این کار ندارد: تنها چیزی که کانفیگی را نگه می‌دارد پنل است.

## حذف نصب

<div dir="ltr" align="left">

    sudo /usr/local/bin/caspian-uninstall


</div>

یا، به همان روشی که نصب‌کننده واکشی می‌شود:

<div dir="ltr" align="left">

    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/uninstall.sh)"


</div>

نصب‌کننده عمداً یک نسخه را روی خودِ دستگاه نگه می‌دارد. کسی که می‌خواهد حذف نصب
کند، اغلب به این دلیل می‌خواهد که شبکهٔ دستگاه در حالتی است که خوشش نمی‌آید، و
همان بدترین لحظه برای نیازداشتن به یک شبکهٔ کارکن جهت واکشی یک اسکریپت است.

هر دو یونیت را متوقف و غیرفعال می‌کند، ژورنال شبکه را بازپخش می‌کند، یونیت‌ها،
باینری و دایرکتوری‌های زمان اجرا را حذف می‌کند، و بعد دربارهٔ وضعیت می‌پرسد. یک
<span dir="ltr">`rm -rf`</span> روی <span dir="ltr">`/run/caspian`</span> دایرکتوری dnsmasq داخلش را هم می‌برد.

| سوییچ | اثر |
|---|---|
| <span dir="ltr">`--dry-run`</span> | هر کنش را چاپ کن، هیچ‌کدام را انجام نده |
| <span dir="ltr">`--purge`</span> | <span dir="ltr">`/var/lib/caspian`</span> را بدون پرسیدن حذف کن. حساب را هم برمی‌دارد |
| <span dir="ltr">`--keep-state`</span> | <span dir="ltr">`/var/lib/caspian`</span> را بدون پرسیدن نگه دار |
| <span dir="ltr">`--force`</span> | حتی وقتی ژورنال شبکه بازپخش نمی‌شود ادامه بده |
| <span dir="ltr">`--show-commands`</span> | هر فرمانِ بازپخش‌شده را کامل چاپ کن. هشدار پایین را ببینید |
| <span dir="ltr">`--yes`</span>، <span dir="ltr">`-y`</span> | هیچ چیز نپرس |

با نبودن <span dir="ltr">`--purge`</span> و <span dir="ltr">`--keep-state`</span> و نبودن ترمینالی برای پرسیدن، وضعیت نگه
داشته می‌شود. یک کانفیگِ حذف‌شده را نمی‌شود پس گرفت و یک کانفیگِ نگه‌داشته‌شده
فقط یک دایرکتوری خرج برمی‌دارد.

حساب کاربری فقط وقتی حذف می‌شود که دایرکتوری وضعیت هم با آن برود. حذف حساب در
حالی که فایل‌هایش هنوز آنجایند، آنها را با مالکیت یک شناسهٔ عددی رها می‌کرد که
حساب بعدیِ ساخته‌شده روی دستگاه می‌توانست آن را به ارث ببرد.

### بازپخش ژورنال

ژورنال در <span dir="ltr">`/var/lib/caspian/netcfg.journal`</span> است. <span dir="ltr">`docs/LAYOUT.md`</span> و
<span dir="ltr">`internal/netcfg/journal.go`</span> (تابع <span dir="ltr">`DefaultJournalPath`</span>) بر سر این نام هم‌نظرند.
تا 2026-08-30 نبودند، چون چیدمان هنوز آن را <span dir="ltr">`teardown.journal`</span> می‌نامید. یک
حذف‌کننده که دنبال نام غلط بگردد، مسیرها، قاعده‌ها و فایروال را بی‌صدا سر جایشان
می‌گذارد و در همان حال به کاربر می‌گوید شبکه بازگردانده شد. پس یک آزمون روی نامی
که اسکریپت دنبالش می‌گردد ادعا می‌کند.

این فایل معکوسِ هر تغییر شبکه‌ای را که سرویس ممتاز داده است در خود دارد. هر معکوس
پیش از آنکه تغییر به کرنل برسد روی دیسک نوشته می‌شود، پس فرایندی که کشته شده و
متوقف نشده است باز هم راه بازگشتی جا می‌گذارد (بخش 5.5 طراحی، و قرارداد <span dir="ltr">`Applier`</span>
در <span dir="ltr">`internal/netcfg/apply.go`</span>).

قالب روی دیسک JSON lines است، یک <span dir="ltr">`Record`</span> در هر سطر، چند سطر برای هر گام، که با
<span dir="ltr">`seq`</span> کلید می‌خورند:

<div dir="ltr" align="left">

    {"seq":2,"phase":"begin","t":"...","op":"route","why":"...",
     "do":{"path":"ip","args":["route","add","203.0.113.7","via","192.168.4.1"]},
     "undo":{"path":"ip","args":["route","del","203.0.113.7","via","192.168.4.1"]}}
    {"seq":2,"phase":"done","t":"..."}


</div>

بازپخشِ حذف‌کننده از قواعد <span dir="ltr">`LoadJournal`</span> و <span dir="ltr">`Entry.NeedsUndo`</span> در
<span dir="ltr">`internal/netcfg/journal.go`</span> پیروی می‌کند، و هر تغییری آنجا باید در بازپخش هم
آینه شود:

- رکورد <span dir="ltr">`begin`</span> مقادیر <span dir="ltr">`op`</span>، <span dir="ltr">`why`</span>، <span dir="ltr">`do`</span> و <span dir="ltr">`undo`</span> را حمل می‌کند. رکوردهای بعدی
  برای همان <span dir="ltr">`seq`</span> فقط فاز را جلو می‌برند.
- یک مدخل تا وقتی آخرین فازش <span dir="ltr">`undone`</span> نباشد هنوز به برگرداندن نیاز دارد. فازهای
  <span dir="ltr">`begin`</span>، <span dir="ltr">`done`</span> و <span dir="ltr">`failed`</span> همه هنوز به آن نیاز دارند، چون فرمانی که در میانهٔ
  کار کشته شده باشد می‌تواند بخشی از اثرش را گذاشته باشد.
- مدخلی که نه <span dir="ltr">`do`</span> دارد و نه <span dir="ltr">`undo`</span> دور ریخته می‌شود.
- معکوس‌ها از تازه‌ترین به قدیمی‌ترین بازپخش می‌شوند.

چهار خاصیتِ این بازپخش عمدی‌اند:

- **فقط و فقط <span dir="ltr">`ip`</span>، <span dir="ltr">`iw`</span>، <span dir="ltr">`nft`</span> یا <span dir="ltr">`sysctl`</span> را اجرا می‌کند.** این همان فهرست
  مجاز در <span dir="ltr">`internal/netcfg/command.go`</span> است، که وجود دارد تا سمت ممتاز هرگز
  فرمانی را که از ورودی کاربر ساخته شده اجرا نکند. همین استدلال با قوت بیشتری
  دربارهٔ فایلی صدق می‌کند که مدتی روی دیسک نشسته است. کل فایل پیش از اجرای هر
  چیزی بررسی می‌شود، پس ژورنالی که مدخل نهمش چیز دیگری را نام می‌برد هشت مدخل
  اولش را هم اجرا نمی‌کند.
- **آرگومان‌ها را چاپ نمی‌کند.** یکی از معکوس‌ها مسیر میزبانِ سنجاق‌شده به سرور
  پروکسی کاربر را حذف می‌کند، پس بردار آرگومانش نشانی آن سرور را در خود دارد، و
  <span dir="ltr">`docs/LAYOUT.md`</span> می‌گوید کانفیگ هرگز چاپ یا ثبت نمی‌شود. خروجی پیش‌فرض شمارهٔ
  توالی و عملیات است، که هر دو از واژگان ثابت در <span dir="ltr">`internal/netcfg/route.go`</span>
  می‌آیند. <span dir="ltr">`--show-commands`</span> بقیه را چاپ می‌کند، و در <span dir="ltr">`--help`</span> می‌گوید که این کار
  را می‌کند.
- **سطری که خوانده نشود رد، شمرده و گزارش می‌شود.** این با <span dir="ltr">`LoadJournal`</span> هم‌خوان
  است که یک دنبالهٔ بریده را دور می‌ریزد به‌جای آنکه هر رکورد کاملِ پیش از آن را
  دور بیندازد. سطرهای رد‌شده بازپخش را ناقص می‌کنند، پس پیام پایانی می‌گوید شبکه
  به‌طور کامل بازگردانده نشد.
- **یک معکوسِ ناموفق جلوی معکوس‌های بعدی را نمی‌گیرد**، هم‌خوان با
  <span dir="ltr">`Applier.Teardown`</span>. یک معکوس معمولاً به این دلیل شکست می‌خورد که چیزی که
  برمی‌گرداند از پیش رفته است.

اگر حذف‌کننده یک ژورنال را یکسره رد کند، هیچ چیز را حذف نمی‌کند و هیچ چیز را
بازپخش نمی‌کند. نرم‌افزار نصب می‌ماند و ژورنال سر جایش می‌ماند، تا کسی بتواند به
مسئله نگاه کند. <span dir="ltr">`--force`</span> این را کنار می‌زند، نرم‌افزار را حذف می‌کند، و به‌روشنی
می‌گوید که شبکه بازگردانده نمی‌شود.

بازپخش به Python نوشته شده است، چون مدخل‌ها JSON‌اند، یک شل بدون کمک نمی‌تواند
JSON را تجزیه کند، و بردار آرگومان باید به <span dir="ltr">`execve`</span> برسد بی‌آنکه در هیچ نقطه‌ای
از یک شل بگذرد. پس برای بازپخش یک ژورنال به <span dir="ltr">`python3`</span> نیاز است. بدون آن،
حذف‌کننده به‌جای حدس‌زدن سر باز می‌زند.

## متغیرهای محیطی

همه برای آزمایش و برای لوله‌کشیِ انتشارند. هیچ‌کدام برای یک نصب عادی لازم نیست.

| متغیر | پیش‌فرض | چه می‌کند |
|---|---|---|
| <span dir="ltr">`CASPIAN_ORG`</span> | <span dir="ltr">`Iman`</span> | مالک مخزن. اگر خالی باشد دانلودها رد می‌شوند |
| <span dir="ltr">`CASPIAN_REPO`</span> | <span dir="ltr">`caspian-byoc`</span> | نام مخزن |
| <span dir="ltr">`CASPIAN_VERSION`</span> | <span dir="ltr">`latest`</span> | یک تگ انتشار، یا <span dir="ltr">`latest`</span> |
| <span dir="ltr">`CASPIAN_BASE_URL`</span> | مشتق‌شده | دایرکتوری انتشار که آرتیفکت و <span dir="ltr">`SHA256SUMS`</span> را دارد. سه مورد بالا را کنار می‌زند |
| <span dir="ltr">`CASPIAN_CHECKSUMS_NAME`</span> | <span dir="ltr">`SHA256SUMS`</span> | نام فایل چک‌سام در آن دایرکتوری |
| <span dir="ltr">`CASPIAN_LOCAL_BINARY`</span> | خالی | به‌جای دانلود، از فایلی روی همین ماشین نصب کن |
| <span dir="ltr">`CASPIAN_LOCAL_CHECKSUMS`</span> | خالی | آن فایل محلی را در برابر این فایل چک‌سام راستی‌آزمایی کن. بدون آن، نصب‌کننده هشدار می‌دهد که دارد راستی‌آزمایی‌نشده نصب می‌کند |
| <span dir="ltr">`CASPIAN_SCRIPT_BASE_URL`</span> | مشتق‌شده | جایی که <span dir="ltr">`uninstall.sh`</span> از آن واکشی می‌شود |
| <span dir="ltr">`CASPIAN_UNINSTALL_SRC`</span> | خالی | به‌جای واکشی، از یک <span dir="ltr">`uninstall.sh`</span> محلی استفاده کن |
| <span dir="ltr">`CASPIAN_ALLOW_INSECURE_URL`</span> | <span dir="ltr">`0`</span> | یک آدرس پایهٔ متن‌ساده را مجاز کن. فقط برای آزمایش |
| <span dir="ltr">`CASPIAN_SYSROOT`</span> | خالی | به هر مسیر مقصد یک پیشوند بده. مگر با <span dir="ltr">`--dry-run`</span> رد می‌شود |
| <span dir="ltr">`CASPIAN_ASSUME_YES`</span> | <span dir="ltr">`0`</span> | همان <span dir="ltr">`--yes`</span> |
| <span dir="ltr">`CASPIAN_SOURCE_ONLY`</span> | <span dir="ltr">`0`</span> | باعث می‌شود <span dir="ltr">`main`</span> بلافاصله برگردد، تا بستر آزمون بتواند هر بار یک تابع را صدا بزند |

## آزمایش کردنش بدون یک انتشار

### اجرای خشک

<div dir="ltr" align="left">

    bash install.sh --dry-run


</div>

هر کنشی را که می‌خواست انجام دهد چاپ می‌کند و هیچ‌کدام را انجام نمی‌دهد. هر تغییر
روی ماشین از یکی از دو تابع <span dir="ltr">`run`</span> و <span dir="ltr">`write_file`</span> می‌گذرد، و هر دو وقتی
<span dir="ltr">`--dry-run`</span> تنظیم باشد به‌جای عمل‌کردن چاپ می‌کنند، و همین است که اجرای خشک را
کامل می‌کند و نه تقریبی.

یک چیز زیر <span dir="ltr">`--dry-run`</span> واقعاً رخ می‌دهد: با تنظیم‌بودنِ <span dir="ltr">`CASPIAN_LOCAL_BINARY`</span>،
آن فایل داخل یک دایرکتوری موقت خصوصی کپی می‌شود و چک‌سامش راستی‌آزمایی می‌شود.
این کار به هیچ چیز بیرون از آن دایرکتوری دست نمی‌زند، و انجام واقعی آن تنها راهی
است که یک اجرای خشک می‌تواند پیش از وجود یک انتشار اثبات کند که راستی‌آزمایی کار
می‌کند.

### یک ماشین ساختگی

رد‌کردن‌ها و نگاشت معماری را <span dir="ltr">`uname`</span> تعیین می‌کند، پس با گذاشتن یک <span dir="ltr">`uname`</span> از آنِ
خودتان در ابتدای <span dir="ltr">`PATH`</span> آزمایش می‌شوند. همراه با <span dir="ltr">`CASPIAN_SYSROOT`</span>، می‌شود یک نصب
کامل را روی ماشینی که Linux نیست قدم زد:

<div dir="ltr" align="left">

    mkdir -p /tmp/fake/bin /tmp/fake/sysroot/run/systemd/system
    printf '%s\n' '#!/bin/sh' \
      'case "${1:-}" in' '  -s) echo Linux ;;' '  -m) echo armv6l ;;' 'esac' \
      > /tmp/fake/bin/uname
    printf '%s\n' '#!/bin/sh' \
      'if [ "${1:-}" = "--version" ]; then echo "systemd 252 (252.1)"; exit 0; fi' \
      'exit 0' > /tmp/fake/bin/systemctl
    chmod 0755 /tmp/fake/bin/*

    env PATH=/tmp/fake/bin:$PATH CASPIAN_SYSROOT=/tmp/fake/sysroot \
      CASPIAN_BASE_URL=https://example.invalid/rel \
      bash install.sh --dry-run --yes


</div>

### راستی‌آزمایی در برابر یک ساخت محلی

<div dir="ltr" align="left">

    go build -o /tmp/caspian-linux-arm64 ./cmd/caspian
    sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

    env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
        CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
        bash install.sh --dry-run --yes


</div>

یک بایت از فایل چک‌سام را تغییر دهید و همان فرمان سر باز می‌زند، هر دو هش را چاپ
می‌کند، و می‌گوید هیچ چیز تغییر نکرده است.

### مجموعهٔ آزمون

<div dir="ltr" align="left">

    bash packaging/test-install.sh


</div>

روی هر ماشینی که bash داشته باشد اجرا می‌شود، از جمله ماشینی که نمی‌شود رویش نصب
کرد. این‌ها را پوشش می‌دهد:

- <span dir="ltr">`bash -n`</span> و <span dir="ltr">`shellcheck`</span> روی هر دو اسکریپت.
- اینکه هیچ فایلِ منتشرشونده‌ای کد escape، ایموجی یا em dash ندارد.
- اینکه یونیت‌های جاسازی‌شده در <span dir="ltr">`install.sh`</span> هنوز بایت‌به‌بایت با <span dir="ltr">`packaging/`</span>
  یکی هستند.
- چهار نگاشت معماری، از جمله اینکه <span dir="ltr">`armv6l`</span> آرتیفکت armv7 نمی‌گیرد.
- شش مسیر رد‌کردن.
- چهار نتیجهٔ ممکن چک‌سام.
- شکل رمز عبورِ تولیدشده.
- بازپخش ژورنال، از جمله اینکه باینریِ بیرون از فهرست مجاز رد و هرگز اجرا نمی‌شود،
  و اینکه نشانی سرور چاپ نمی‌شود.
- یک اجرای خشک کامل، هم برای نصب تازه و هم برای ارتقا.

### آنچه این آزمون‌ها پوشش نمی‌دهند

هر چه پایین می‌آید به یک Raspberry Pi نیاز دارد و هیچ‌کدام اجرا نشده است:

- اینکه آیا <span dir="ltr">`caspian.service`</span> سخت‌سازی‌شده واقعاً شروع می‌شود، و اینکه آیا
  <span dir="ltr">`SystemCallFilter=@system-service`</span>، <span dir="ltr">`RestrictAddressFamilies`</span> یا
  <span dir="ltr">`DevicePolicy=closed`</span> جلوی چیزی را می‌گیرند که واقعاً لازم است. فهرست
  capabilityهای لازم از روی کد استدلال شده است، نه اندازه‌گرفته‌شده.
- اینکه آیا بارگذاری بر حسب نیازِ ماژول nftables داخل یونیت روی یک بوت تازه کار
  می‌کند، و اینکه آیا <span dir="ltr">`/etc/modules-load.d/caspian.conf`</span> به‌اندازهٔ کافی آن را
  پوشش می‌دهد.
- اینکه آیا نام بسته‌ها روی چیزی جز دبیان درست است.
- اینکه آیا <span dir="ltr">`systemd-tmpfiles --create`</span> روی دستگاه هدف <span dir="ltr">`/run/caspian`</span> را با
  <span dir="ltr">`root:caspian 0750`</span> تولید می‌کند.
- اینکه آیا یک <span dir="ltr">`sha256sum`</span> واقعی روی Pi و فایل چک‌سامی که خط لولهٔ انتشار تولید
  می‌کند در قالب با هم می‌خوانند.
- هر چیزی دربارهٔ پنل: هیچ چیز هنوز رمز نخستین اجرا را مصرف نمی‌کند، و نشان داده
  نشده است که نشانی چاپ‌شده نشانی‌ای باشد که پنل روی آن پاسخ دهد.
- اینکه آیا ژورنالی که یک اجرای واقعی نوشته باشد تمیز بازپخش می‌شود. بازپخش در
  برابر <span dir="ltr">`internal/netcfg/journal.go`</span> نوشته شده و در برابر داده‌های آزمونی به همان
  شکل آزموده شده است، اما هیچ ژورنالی که <span dir="ltr">`Applier`</span> واقعی روی یک دستگاه واقعی
  تولید کرده باشد از آن نگذشته است.

## پیش از نخستین انتشار

چهار چیز باید پیش از کارکردن نصب تک‌سطری حل‌وفصل می‌شد. سه‌تا در گردش‌کار انتشار
حل شدند و چهارمی در کد.

1. **مالک مخزن.** <span dir="ltr">`Iman`</span>. مقدار پیش‌فرض <span dir="ltr">`CASPIAN_ORG`</span> در <span dir="ltr">`install.sh`</span> همین است و
   آدرس‌های انتشار به همین می‌رسند.
2. **<span dir="ltr">`GOARM=6`</span> برای آرتیفکت <span dir="ltr">`linux/arm`</span>.** گردش‌کار آن را همان‌طور می‌سازد و بعد
   نتیجه را با <span dir="ltr">`readelf`</span> بررسی می‌کند و اگر آرتیفکت ARMv6 نباشد انتشار را شکست
   می‌دهد. هم ماشین‌های <span dir="ltr">`armv6l`</span> و هم <span dir="ltr">`armv7l`</span> همان یک فایل را نصب می‌کنند، پس یک
   ساخت ARMv7 هر Pi 1، Zero و Zero W را با یک illegal instruction در اولین اجرا
   می‌کشت. این یک بار پیش‌تر در همین کارگاه رخ داده است، و به همین دلیل به‌جای
   کامنتی که تقاضای دقت کند، این بررسی وجود دارد.
3. **نام فایل چک‌سام.** <span dir="ltr">`SHA256SUMS`</span>، در قالب <span dir="ltr">`sha256sum`</span>، که کنار آرتیفکت‌ها
   منتشر می‌شود. این انتخابِ همین نصب‌کننده است، پس گردش‌کار طوری نوشته شده که با
   <span dir="ltr">`CASPIAN_CHECKSUMS_NAME`</span> بخواند و نه برعکس.
4. **تحویل رمز نخستین اجرا.** انجام شد: <span dir="ltr">`cmd/caspian/firstrun.go`</span> تابع
   <span dir="ltr">`consumeFirstRunPassword`</span> را فراهم می‌کند، <span dir="ltr">`cmd/caspian/serve_panel.go`</span> آن را
   در شروع پنل صدا می‌زند، و <span dir="ltr">`cmd/caspian/firstrun_test.go`</span> پوششش می‌دهد. رمزی که
   چاپ می‌شود کار می‌کند.

آنچه هنوز درست است: هیچ انتشاری هنوز بیرون نیامده است. تا وقتی یک تگ نسخه push
نشود، <span dir="ltr">`releases/latest`</span> به هیچ چیز نمی‌رسد و نصب تک‌سطری چیزی برای واکشی ندارد.

</div>
