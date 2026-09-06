<div dir="rtl" align="right">

# نصب

[English](https://github.com/Iman/caspian/wiki/Installation) | [فارسی](https://github.com/Iman/caspian/wiki/Installation.fa) | [Русский](https://github.com/Iman/caspian/wiki/Installation.ru) | [中文](https://github.com/Iman/caspian/wiki/Installation.zh)

[ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa)

> این راهنما از README موجود منتقل شده است. تاریخ اندازه‌گیری‌ها همان تاریخ اصلی است؛ این جابه‌جایی گزارش اجرای تازهٔ آزمون‌ها نیست.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## نصب در Windows 10 و 11

پشتیبانی از Windows 10 آزمایشی است و هنوز تست نشده است. کد فعلی روی x64 به
نسخهٔ 2004 (بیلد 19041) یا جدیدتر نیاز دارد. سازگاری با Windows 10 روی ARM64
هنوز تأیید نشده است.

برنامهٔ نصب همهٔ چیزهای لازم را نصب می‌کند. به PowerShell، Go یا .NET SDK نیاز
ندارید.

### چه چیزهایی لازم دارید

- یک کامپیوتر با Windows 11، یا Windows 10 نسخهٔ 2004 یا جدیدتر روی x64 (آزمایشی).
- یک حساب مدیر روی همان کامپیوتر.
- یک آداپتور وای‌فای که Windows Mobile Hotspot را پشتیبانی کند.
- اتصال اینترنت.
- یک لینک یا پیکربندی پروکسی که Caspian پشتیبانی کند.

### فایل درست را انتخاب کنید

بیشتر کامپیوترهای Intel و AMD از این فایل استفاده می‌کنند:

- <span dir="ltr">`CaspianSetup-0.2.1-windows-x64.exe`</span>

کامپیوترهای Windows با پردازندهٔ Snapdragon یا ARM از این فایل استفاده می‌کنند:

- <span dir="ltr">`CaspianSetup-0.2.1-windows-arm64.exe`</span>

اگر نوع کامپیوتر را نمی‌دانید، **Settings** را باز کنید. **System** و سپس
**About** را انتخاب کنید. نوع دستگاه در بخش **System type** نوشته شده است.

### نصب Caspian

1. [صفحهٔ انتشار Caspian](https://github.com/Iman/caspian/releases/latest) را باز کنید.
2. بخش **Assets** آخرین نسخه را باز کنید.
3. فایل مناسب Windows را دانلود کنید.
4. روی فایل دانلودشده دو بار کلیک کنید.
5. اگر SmartScreen باز شد، **More info** را انتخاب کنید.
6. مطمئن شوید نام فایل در هشدار همان نام فایل دانلودشده است.
7. **Run anyway** را انتخاب کنید.
8. وقتی Windows اجازهٔ مدیر می‌خواهد، **Yes** را انتخاب کنید.
9. صفحهٔ مجوز را بخوانید و ادامه دهید.
10. یک رمز برای پنل وب Caspian انتخاب کنید.
11. همان رمز را دوباره وارد کنید.
12. این رمز را در جای امن نگه دارید.

نصب‌کننده دو انتخاب اختیاری نیز نشان می‌دهد:

- ساخت میان‌بر روی دسکتاپ.
- اجرای Caspian Control هنگام ورود به Windows.

هر دو انتخاب به‌طور پیش‌فرض خاموش‌اند. نصب‌کننده همیشه میان‌بر **Caspian
Control** را در منوی Start می‌سازد. در پایان، **Open Caspian Control** را روشن
بگذارید و **Finish** را انتخاب کنید.

Windows تا زمانی که فایل گواهی امضای کد ندارد، هشدار **Unknown publisher** را
نشان می‌دهد. فقط فایل صفحهٔ رسمی انتشار Caspian را اجرا کنید.

![Caspian Control در Windows](https://github.com/Iman/caspian/blob/main/docs/images/caspian-control-windows.png)

### دو صفحهٔ Caspian

Caspian در Windows دو صفحهٔ جدا دارد:

| صفحه | کجا باز می‌شود | چه کاری انجام می‌دهد |
|---|---|---|
| **Caspian Control** | برنامهٔ کوچک Windows و آیکون کنار ساعت | سرویس‌های پس‌زمینهٔ Caspian را شروع، متوقف یا دوباره راه‌اندازی می‌کند |
| **پنل وب Caspian** | مرورگر در <span dir="ltr">`http://127.0.0.1:8088/`</span> | نام و رمز وای‌فای، باند و اتصال پروکسی را تنظیم می‌کند |

ابتدا از **Caspian Control** استفاده کنید. وقتی **Ready** ظاهر شد، **Open
panel** را انتخاب کنید. پنل وب صفحهٔ دوم است. هات‌اسپات و تونل را در این صفحه
راه‌اندازی می‌کنید.

وضعیت **Ready** در Caspian Control فقط یعنی دو سرویس پس‌زمینه پاسخ می‌دهند. این
وضعیت به معنی اتصال تونل نیست. وقتی هات‌اسپات و تونل آماده‌اند، پنل وب سبز
می‌شود.

### اولین اجرا

1. **Caspian Control** را از منوی Start یا دسکتاپ باز کنید.
2. وقتی Windows اجازهٔ مدیر می‌خواهد، **Yes** را انتخاب کنید.
3. **Start all** را انتخاب کنید.
4. صبر کنید تا کارت بزرگ **Ready** را نشان دهد.
5. **Open panel** را انتخاب کنید.
6. رمزی را وارد کنید که هنگام نصب انتخاب کردید.
7. **Sign in** را انتخاب کنید.
8. یک نام برای شبکهٔ وای‌فای جدید وارد کنید.
9. یک رمز وای‌فای با دست‌کم هشت نویسه وارد کنید.
10. برای دستگاه‌های قدیمی، **2.4 GHz** را نگه دارید.
11. لینک یا پیکربندی پروکسی را پیست کنید.
12. کلید شروع Caspian را روشن کنید.
13. صبر کنید تا وضعیت پنل وب سبز شود.
14. تلفن یا دستگاه دیگر را به شبکهٔ وای‌فای جدید وصل کنید.
15. یک وب‌سایت را روی همان دستگاه باز کنید و اتصال را آزمایش کنید.

پنل دستگاه‌های متصل را نشان می‌دهد. Windows به آن‌ها نشانی‌هایی از
<span dir="ltr">`192.168.137.0/24`</span> می‌دهد. Caspian ترافیک اینترنت آن‌ها را از تونل <span dir="ltr">`xray0`</span>
می‌فرستد.

رمز پنل و رمز وای‌فای دو رمز جدا هستند. رمز پنل، پنل وب را باز می‌کند. رمز
وای‌فای، تلفن و دستگاه‌های دیگر را به شبکه وصل می‌کند.

### دکمه‌های Caspian Control

| دکمه | نتیجه |
|---|---|
| **Start all** | هر دو سرویس Caspian را شروع می‌کند |
| **Stop all** | هر دو سرویس را متوقف نگه می‌دارد |
| **Restart all** | هر دو سرویس را متوقف و دوباره شروع می‌کند |
| **Open panel** | پنل وب Caspian را در مرورگر باز می‌کند |

بعد از بستن پنجره، برنامه کنار ساعت Windows می‌ماند. برای باز کردن دوباره، روی
آیکون Caspian دو بار کلیک کنید.

Windows هنگام توقف یا شروع دوبارهٔ هات‌اسپات، دستگاه‌ها را قطع می‌کند. صبر کنید
تا پنل وب سبز شود، سپس دستگاه‌ها را دوباره وصل کنید.

اگر Caspian Control وضعیت **Ready** دارد ولی پنل وب قرمز است، پیام پنل وب را
بخوانید. پنل وب هات‌اسپات و تونل پروکسی را آزمایش می‌کند.

### نصب برای توسعه‌دهندگان

روش PowerShell فقط برای توسعه‌دهندگان است. ابتدا
[Go](https://go.dev/dl/)، [.NET 9 SDK](https://dotnet.microsoft.com/download/dotnet/9.0)
و [Git for Windows](https://git-scm.com/download/win) را نصب کنید. سپس PowerShell
را با دسترسی مدیر باز کنید و این فرمان را اجرا کنید:

<div dir="ltr" align="left">

    powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\YOUR-NAME\Documents\caspian\packaging\windows\install.ps1" -NoOpen


</div>

گزینهٔ <span dir="ltr">`-NoOpen`</span> از باز شدن خودکار پنل جلوگیری می‌کند. برنامه‌های Windows از
[.NET runtime and Windows Forms](https://github.com/dotnet/runtime) و
<span dir="ltr">`System.ServiceProcess.ServiceController`</span> استفاده می‌کنند. مجوزها و اعلان‌های
.NET در <span dir="ltr">`third_party/dotnet/`</span> قرار دارند. مجوز Wintun در
[<span dir="ltr">`third_party/wintun/PREBUILT-BINARIES-LICENSE.txt`</span>](https://github.com/Iman/caspian/blob/main/third_party/wintun/PREBUILT-BINARIES-LICENSE.txt) قرار دارد و نسخهٔ اصلی آن از
[وب‌سایت Wintun](https://www.wintun.net/) دریافت می‌شود.

سرویس <span dir="ltr">`caspian-panel`</span> پنل وب را اجرا می‌کند. فایل <span dir="ltr">`caspian.exe`</span> تونل را می‌سازد.
فایل <span dir="ltr">`caspian-tethering.exe`</span> هات‌اسپات Windows را کنترل می‌کند. برنامهٔ
<span dir="ltr">`CaspianControl.exe`</span> سرویس‌ها را کنترل می‌کند و <span dir="ltr">`wintun.dll`</span> رابط تونل را فراهم
می‌کند.

## نصب در macOS 13 یا جدیدتر

فایل DMG برنامهٔ بومی **Caspian Control** و موتور Caspian را در خودش دارد. به
Terminal، Go، Homebrew یا runtime دیگری نیاز ندارید. به یک حساب مدیر نیاز دارید
و اگر وای‌فای داخلی قرار است هات‌اسپات باشد، اینترنت باید از Ethernet سیمی وارد
Mac شود.

### فایل درست را انتخاب کنید

- Macهای Intel از <span dir="ltr">`Caspian-v0.2.4-macos-amd64.dmg`</span> استفاده می‌کنند.
- Macهای Apple Silicon، یعنی M1 و جدیدتر، از
  <span dir="ltr">`Caspian-v0.2.4-macos-arm64.dmg`</span> استفاده می‌کنند.

اگر نوع پردازنده را نمی‌دانید، **Apple menu → About This Mac** را باز کنید.

### نصب و اجازه دادن به نخستین اجرا

برنامهٔ نسخهٔ v0.2.4 امضای ad-hoc دارد، اما هنوز با Apple Developer ID امضا و
توسط Apple notarize نشده است. به همین دلیل Gatekeeper پیام **“Caspian” Not
Opened** را نشان می‌دهد و می‌گوید Apple نتوانسته بی‌ضرر بودن برنامه را تأیید
کند. این پیام به معنی crash کردن برنامه نیست. فقط برای فایلی که از صفحهٔ رسمی
انتشار Caspian گرفته‌اید این محدودیت را بردارید.

1. [آخرین انتشار Caspian](https://github.com/Iman/caspian/releases/latest) را باز
   و بخش **Assets** را باز کنید.
2. DMG مناسب پردازندهٔ Mac را دانلود و باز کنید.
3. <span dir="ltr">`Caspian.app`</span> را داخل پوشهٔ **Applications** بکشید.
4. نسخه‌ای را که در **Applications** است یک بار باز کنید.
5. وقتی Gatekeeper آن را مسدود کرد، **Done** را بزنید.
6. مسیر **Apple menu → System Settings → Privacy & Security** را باز کنید.
7. تا بخش **Security** پایین بروید و کنار Caspian روی **Open Anyway** بزنید. این
   دکمه پس از تلاش ناموفق برای باز کردن برنامه حدود یک ساعت نمایش داده می‌شود.
8. رمز ورود Mac را وارد کنید، **OK** را بزنید و سپس **Open** را تأیید کنید.

macOS این برنامه را به‌عنوان یک استثنا نگه می‌دارد و دفعات بعد با دوبار کلیک
عادی باز می‌شود. همین فرایند در راهنمای رسمی Apple با عنوان
[باز کردن برنامه با تغییر تنظیم امنیتی](https://support.apple.com/guide/mac-help/apple-cant-check-app-for-malicious-software-mchleab3a043/26/mac/26)
توضیح داده شده است.

### اگر macOS همچنان سرویس پس‌زمینه را مسدود می‌کند

فایل اجرایی نصب‌شدهٔ سرویس پس‌زمینه، <span dir="ltr">`/usr/local/bin/caspian`</span>، ممکن است پس از
تأیید <span dir="ltr">`Caspian.app`</span> همچنان ویژگی قرنطینه را داشته باشد. در این حالت هشدار نام
<span dir="ltr">`caspian`</span> را با حروف کوچک نشان می‌دهد و پنجرهٔ کنترل ممکن است پیام
**Caspian needs attention** را نمایش دهد.

**اگر هشدار نام تروجان را ذکر می‌کند یا از شناسایی بدافزار خبر می‌دهد، دستور زیر را اجرا نکنید.**
نصب را متوقف کنید و متن دقیق هشدار، نام تهدید شناسایی‌شده، نسخهٔ انتشار و
نشانی دانلود را در یک [گزارش GitHub](https://github.com/Iman/caspian/issues) بنویسید.
شناسایی بدافزار به بررسی نیاز دارد؛ نداشتن امضای معتبر به‌تنهایی ثابت نمی‌کند
که تشخیص اشتباه است. به
[توضیح Apple دربارهٔ هشدارهای امنیتی macOS](https://support.apple.com/en-ie/102445)
مراجعه کنید.

فقط برای هشدار تأیید نشدن توسعه‌دهنده یا notarize نشدن برنامه، و پس از اطمینان
به فایل و منبع آن، از این روش جایگزین استفاده کنید. فایل انتشار را از صفحهٔ
رسمی انتشار Caspian بگیرید و چک‌سام DMG را با <span dir="ltr">`SHA256SUMS`</span> همان انتشار مقایسه
کنید. تطابق چک‌سام، یکسان بودن فایل با فایل انتشار را تأیید می‌کند، نه ایمن بودن آن را.

1. **Terminal** را باز کنید.
2. ویژگی قرنطینه را از فایل اجرایی نصب‌شدهٔ سرویس پس‌زمینه بردارید:

   <div dir="ltr" align="left">

   ```bash
   sudo xattr -d com.apple.quarantine /usr/local/bin/caspian
   ```

   </div>

3. رمز ورود Mac را وارد کنید. Terminal هنگام تایپ، رمز را نمایش نمی‌دهد.
4. در Caspian، **Advanced options → Restart services** را انتخاب کنید.

این دستور فقط ویژگی قرنطینهٔ فایل مشخص‌شده را حذف می‌کند. فایل اجرایی را
اسکن، امضا یا notarize نمی‌کند. اگر Terminal پیام <span dir="ltr">`No such xattr`</span> را نشان داد،
این ویژگی از قبل وجود ندارد. اگر سرویس همچنان اجرا نمی‌شود، خطا را گزارش کنید
و سایر کنترل‌های امنیتی را غیرفعال نکنید.

### بگذارید Caspian خودش نصب را انجام دهد و رمز را نگه دارید

1. **Caspian Control** را اجرا کنید. برنامه پیش از بررسی پنل، سرویس پس‌زمینهٔ
   داخل خودش را با نسخهٔ نصب‌شده مقایسه می‌کند.
2. در نخستین اجرا، یا وقتی DMG به‌روزرسانی دارد، نصب خودکار آغاز می‌شود. در
   پنجرهٔ اجازهٔ macOS رمز مدیر را وارد کنید. اگر نسخهٔ نصب‌شده همان نسخهٔ داخل
   برنامه باشد، دیگر رمزی درخواست نمی‌شود.
3. صبر کنید تا پنجرهٔ کنترل عبارت **Caspian is ready** را نشان دهد. اگر اجازه را
   لغو کردید، دکمهٔ **Set up Caspian** یا **Update Caspian** برای تلاش دوباره
   باقی می‌ماند.
4. در نصب نخست، **first-run panel password** نشان‌داده‌شده در خروجی را در جایی
   امن نگه دارید. دکمهٔ **Copy panel password** فقط همان رمز را کپی می‌کند.
5. **Open panel** را بزنید و با رمز پنلی که نگه داشتید وارد شوید.
6. نام و رمز وای‌فای را وارد کنید، کانفیگ پروکسی را پیست کنید و کلید پنل را برای
   شروع Caspian بزنید.

رمز ورود Mac، رمز پنل Caspian و رمز وای‌فای سه رمز جدا هستند. اگر رمز پنل را
گم کردید، از **Reset password** در Caspian Control استفاده کنید؛ این کار به
اجازهٔ مدیر نیاز دارد اما کانفیگ پروکسی و تنظیمات هات‌اسپات را نگه می‌دارد. بستن
پنجرهٔ کنترل، آیکون آن را در نوار بالای macOS فعال نگه می‌دارد؛ برای باز کردن
دوباره، از همان منو **Open Caspian Control** را انتخاب کنید.

## نصب در Linux و Raspberry Pi

دو راه. اولی برای کسی است که می‌خواهد آن را اجرا کند، دومی برای کسی که می‌خواهد
اول وارسی‌اش کند.

### خودکار: یک خط

<div dir="ltr" align="left">

    sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"


</div>

نصب‌کننده تشخیص می‌دهد روی چه دستگاهی است، فایل اجراییِ متناظر را از آخرین
انتشار می‌گیرد، و اگر آنچه دانلود شد با چک‌سامِ منتشرشده نخواند، کار را رد
می‌کند.

| <span dir="ltr">`uname -m`</span> | فایل | دستگاهِ معمول |
|---|---|---|
| <span dir="ltr">`x86_64`</span> | <span dir="ltr">`caspian-linux-amd64`</span> | یک لپ‌تاپ یا یک مینی‌پی‌سی |
| <span dir="ltr">`aarch64`</span> | <span dir="ltr">`caspian-linux-arm64`</span> | Raspberry Pi 3، 4، 5 روی سیستم 64 بیتی |
| <span dir="ltr">`armv7l`</span> | <span dir="ltr">`caspian-linux-arm`</span> | Raspberry Pi 2 و 3 روی سیستم 32 بیتی |
| <span dir="ltr">`armv6l`</span> | <span dir="ltr">`caspian-linux-arm`</span> | Raspberry Pi 1، Zero، Zero W |

وقتی مطمئن نیست حدس نمی‌زند، بلکه رد می‌کند. لینوکس نبودن، معماری‌ای که در آن
جدول نیست، نبودِ systemd، یا چک‌سامی که نمی‌خواند: هر کدام یک ردکردن است که
می‌گوید چه دیده. <span dir="ltr">`armv8l`</span>، یعنی فضای کاربریِ 32 بیتی روی هستهٔ 64 بیتی، عمداً
به هیچ فایلی نگاشت نشده است، چون حدس زدن در همان‌جا بود که یک پروژهٔ قبلی کدِ
ARMv7 را روی دستگاه‌های ARMv6 فرستاد و آن‌ها در نخستین اجرا با دستورالعمل
غیرمجاز مردند.

**اسکریپت را پیش از آنکه به یک shell بدهید بخوانید.** برای نرم‌افزاری از این
جنس، این توصیه تشریفات نیست، و اسکریپت طوری نوشته شده که خوانده شود.

<div dir="ltr" align="left">

    curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh | less


</div>

برای سنجاق کردن یک نسخهٔ مشخص به‌جای گرفتن آخرین نسخه:

<div dir="ltr" align="left">

    sudo env CASPIAN_VERSION=v0.2.5 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"


</div>

### وارسی کردن دانلود، خودتان

هر انتشار یک فایل <span dir="ltr">`SHA256SUMS`</span> همراه دارد. نصب‌کننده آن را برای شما بررسی
می‌کند، و شما هم می‌توانید مستقل بررسی کنید:

<div dir="ltr" align="left">

    curl -fsSLO https://github.com/Iman/caspian/releases/latest/download/caspian-linux-arm64
    curl -fsSLO https://github.com/Iman/caspian/releases/latest/download/SHA256SUMS
    sha256sum -c SHA256SUMS --ignore-missing


</div>

این چه چیزی را ثابت می‌کند و چه چیزی را نه: ثابت می‌کند فایلی که دارید همان
فایلی است که آن انتشار منتشر کرده. ثابت نمی‌کند چه کسی آن انتشار را ساخته است.
فایل‌های اجرایی را GitHub Actions از یک کامیتِ تگ‌خورده می‌سازد، و گردش‌کاری که
آن‌ها را می‌سازد در همین مخزن است، در [<span dir="ltr">`.github/workflows/release.yml`</span>](https://github.com/Iman/caspian/blob/main/.github/workflows/release.yml). پس ساخت
خواندنی است، هرچند به‌طور مستقل بازتولیدپذیر نیست.

### دستی: خودتان بسازید

هیچ‌چیزِ راهِ خودکار اجباری نیست. ساخت از روی کد به Go 1.26 یا بالاتر نیاز دارد
و فایل اجرایی‌ای می‌دهد که در عملکرد یکی است.

<div dir="ltr" align="left">

    git clone https://github.com/Iman/caspian.git
    cd caspian
    go build -trimpath -o caspian ./cmd/caspian
    sudo CASPIAN_LOCAL_BINARY="$PWD/caspian" bash install.sh


</div>

<span dir="ltr">`CASPIAN_LOCAL_BINARY`</span> به نصب‌کننده می‌گوید به‌جای دانلود، از فایلی که همین حالا
ساخته‌اید استفاده کند. باقی کارهای نصب‌کننده، یعنی ساختن حساب سرویس و پوشه‌ها و
یونیت‌ها و دسترسی‌هایشان، به همان شکل انجام می‌شود.

کامپایل متقابل برای یک Pi از روی دستگاهی دیگر:

<div dir="ltr" align="left">

    GOOS=linux GOARCH=arm64 go build -trimpath -o caspian-linux-arm64 ./cmd/caspian
    GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -o caspian-linux-arm ./cmd/caspian


</div>

**<span dir="ltr">`GOARM=6`</span> در ساخت 32 بیتی اختیاری نیست.** دستگاه‌های <span dir="ltr">`armv6l`</span> و <span dir="ltr">`armv7l`</span> هر دو
همان فایل <span dir="ltr">`arm`</span> را نصب می‌کنند، پس یک ساختِ ARMv7 هر Pi 1 و Zero و Zero W ای را
که آن را نصب کند از کار می‌اندازد. گردش‌کارِ انتشار این را با <span dir="ltr">`readelf`</span> بررسی
می‌کند و به‌جای منتشر کردن فایلی که دربارهٔ معماریِ خودش دروغ می‌گوید، شکست
می‌خورد.

پیش از آنکه به یک ساخت اعتماد کنید، دروازه را اجرا کنید:

<div dir="ltr" align="left">

    bash scripts/gate.sh


</div>

این دروازه قالب‌بندی، <span dir="ltr">`go vet`</span>، کل مجموعهٔ آزمون همراه با race detector، کف
پوششِ هر بسته، لایهٔ رگرسیونِ golden، یک پویش حریم خصوصی و یک زیرمجموعهٔ smoke را
اجرا می‌کند. در صورت شکست با کد غیرصفر بیرون می‌آید. آن را به هیچ لوله‌ای ندهید:
یک لولهٔ shell وضعیتِ آخرین فرمانش را گزارش می‌کند، پس دادنش به <span dir="ltr">`tail`</span> همان
جوابی را که خواسته بودید دور می‌ریزد.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

</div>
