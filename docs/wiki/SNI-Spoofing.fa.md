<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/SNI-Spoofing) · [**فارسی**](https://github.com/Iman/caspian/wiki/SNI-Spoofing.fa) · [Русский](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ru) · [简体中文](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh) · [العربية](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr) · [اردو](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ur)

</div>

<div dir="rtl" lang="fa">

[ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa) · [عیب‌یابی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa)

</div>
<!-- wiki-navigation:end -->

<div dir="rtl" align="right">

<a id="caspian-sni-spoofing-and-tls-splitting-for-dpi-circumvention"></a>
# جعل کاسپین SNI و تقسیم TLS برای دور زدن DPI

این راهنما دور زدن اختیاری بازرسی بسته عمیق (DPI) را در `feature/sni` پوشش می‌دهد، نه نصب‌کننده فعلی منتشر شده.

SNI نام سرور در یک تبریک TLS است.
این ویژگی یک تبریک اضافی با نام جعلی قبل از جریان پروکسی واقعی ارسال می کند.
نام واقعی TLS یا REALITY در پیکربندی وارد شده شما بدون تغییر باقی می ماند.
یک مقدار `sni` وارد شده به طور خودکار جعل را فعال نمی کند.

<a id="set-a-spoof-name"></a>
## یک نام جعلی تنظیم کنید

1. تنظیمات پروکسی خود را در پنل ذخیره کنید.
2. **دور زدن DPI (اختیاری)** را در داشبورد باز کنید.
3. یک نام دامنه مانند `cover.example.invalid` برای آزمایش محلی وارد کنید.
4. تنظیم را ذخیره کنید.

از یک دامنه مناسب برای شبکه واقعی خود استفاده کنید. دامنه مثال نمی تواند حل شود.
اگر Caspian روشن است، ذخیره آن را با تنظیمات جدید دوباره وصل می کند.
برای غیرفعال کردن جعل، فیلد را پاک کرده و ذخیره کنید.
با جایگزین کردن پیکربندی، نام جعلی و هر دو تنظیمات تقسیم پاک می شود.
انتخاب ورودی دیگر یا تازه کردن یک اشتراک آن را حفظ می کند.

<a id="independent-tcp-split-and-tls-record-split"></a>
## تقسیم TCP مستقل و تقسیم رکورد TLS

**دور زدن DPI (اختیاری)** را در کنار پیکربندی ذخیره شده خود باز کنید:

- **نام سرور جعلی**: برای خاموش کردن SNI جعلی، آن را خالی بگذارید.
- **تقسیم TCP**: تبریک اولیه TLS را در دو نوشته نزدیک به وسط نام میزبان SNI ارسال کنید.
- **تقسیم رکورد TLS**: یک رکورد تبریک TLS را در آن موقعیت بدون تغییر محتویات دست دادن تقسیم کنید.

هر گزینه مستقل است. می توانید آنها را ترکیب کنید یا هر سه را کنار بگذارید.
بدون پسوند SNI، تقسیم از موقعیتی درست بعد از نوع دست دادن استفاده می کند.
تقسیم TCP بهترین تلاش است: نوشتن جداگانه بسته های جداگانه را در هر سیستم عامل و شبکه تضمین نمی کند.
تقسیم رکورد TLS مرزهای رکورد را تغییر می دهد و ممکن است در برخی از سرورها یا CDN ها با شکست مواجه شود.
هر دو گزینه تقسیم در حال حاضر به TLS معمولی با VLESS، VMess، یا Trojan از طریق انتقال IPv4 TCP پشتیبانی شده نیاز دارند.
تقسیم REALITY فعال نیست. REALITY هنوز هم می تواند از SNI جعلی به تنهایی استفاده کند.
فقط ClientHello اولیه تقسیم می شود. ترافیک برنامه بعدی به طور معمول رله می شود.
حالت فقط تقسیم به WinDivert، سوکت بسته یا BPF نیاز ندارد.
هنگامی که SNI جعلی نیز انتخاب می شود، تأیید بسته آن باید قبل از ارسال هرگونه تبریک واقعی به پایان برسد.
احوالپرسی ناقص، ناقص، بزرگ یا متوقف شده، اتصال را می بندد. بدون تلاش مجدد بی سر و صدا یک گزینه فعال را حذف می کند.
ذخیره یک تونل فعال را دوباره وصل می کند. جایگزینی پیکربندی هر سه گزینه را پاک می کند.

فیلدهای حالت `spoof_sni`، `tcp_split` و `tls_record_split` هستند.
ارتقاء یک فایل نسخه 4 نام جعلی خود را حفظ می کند و هر دو تنظیمات تقسیم را خاموش می کند.

<a id="state-compatibility"></a>
## سازگاری دولت

این شعبه نسخه 5 حالت طرحواره را می نویسد.
بیلدهای قدیمی برای جلوگیری از از دست دادن اطلاعات، این فایل حالت را رد می کنند.
در صورت نیاز به بازگشت به یک ساخت قدیمی، یک نسخه پشتیبان از وضعیت قبل از ارتقا نگه دارید.

<a id="limits"></a>
## محدودیت ها

این نسخه از VLESS، VMess و Trojan از طریق IPv4 TCP، از جمله WebSocket، HTTPUpgrade و gRPC پشتیبانی می کند.
از سرورهای Hysteria2، QUIC، SOCKS، Shadowsocks، XHTTP یا فقط IPv6 پشتیبانی نمی کند.
SOCKS می تواند یک نقطه پایانی جداگانه UDP را که این ارسال کننده TCP نمی تواند پوشش دهد، مذاکره کند.
Shadowsocks UDP بومی، از جمله ترافیک DNS تونل، نمی تواند از این ارسال کننده فقط TCP استفاده کند.
ارسال کننده اولین آدرس IPv4 موجود را از آدرس های سرور شناسایی شده انتخاب می کند.
اگر تأیید ناموفق باشد، بدون ارسال جریان واقعی، اتصال را می‌بندد.
به یک اتصال معمولی باز نمی گردد.

SNI جعلی در ویندوز x64 به فایل‌های WinDivert موجود در ساخت نصب کننده محلی نیاز دارد.
ویندوز ARM64 نمی تواند از SNI جعلی استفاده کند. حالت فقط تقسیم از باطن بسته استفاده نمی کند.
SNI جعلی در لینوکس به امتیازات سوکت بسته نیاز دارد. macOS نیاز به دسترسی به یک دستگاه BPF در یک رابط به سبک اترنت دارد.
سرویس ممتاز کاسپین مالک این منابع است و در صورت توقف آنها را می بندد.

تست لوپ بک ویندوز ثابت می کند که سرور جریان واقعی را بدون تغییر دریافت می کند.
این ثابت نمی کند که جعل بر خلاف فیلتر ارائه دهنده شما عمل می کند.
ساخت‌های لینوکس و macOS نیز به آزمایش بسته‌های زنده روی سیستم‌های هدف خود نیاز دارند.

<a id="credits"></a>
## اعتبارات

منبع و ایده اولیه [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) هستند که تحت مجوز GPL-3.0 هستند.
[اعتبارات شخص ثالث](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md) را ببینید.

<a id="dpi-bypass-sni-spoofing-and-security"></a>
## بای پس DPI، جعل SNI، و امنیت

<a id="is-caspian-dpi-safe"></a>
### آیا DPI کاسپین ایمن است؟

هیچ تضمین جهانی ایمن DPI وجود ندارد. جعل اختیاری SNI تلاش می‌کند بر نحوه خواندن ترافیک اولیه TCP توسط یک سیستم فیلتر تأثیر بگذارد.
IP سرور، حجم ترافیک یا زمان‌بندی را پنهان نمی‌کند و ارائه‌دهنده همچنان می‌تواند اتصال را مسدود کند.
پیاده سازی `feature/sni` هویت واقعی TLS را حفظ می کند و به جای ارسال مستقیم جریان واقعی، تأیید جعلی ناموفق را رد می کند.

<a id="does-caspian-include-goodbyedpi-or-zapret"></a>
### آیا کاسپین شامل GoodbyeDPI یا zapret می شود؟

شماره [GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) و [zapret](https://github.com/bol-van/zapret) مرجع تحقیقاتی برای استراتژی های احتمالی آینده هستند.
کد و ایده اولیه SNI از [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) آمده است، با حفظ انتساب GPL.
کاسپین آن پروژه‌های دیگر را جمع نمی‌کند و ادعا نمی‌کند که نویسندگان آن‌ها را تأیید می‌کنند.

<a id="does-a-config-with-sni-enable-dpi-bypass-automatically"></a>
### آیا پیکربندی با SNI دور زدن DPI را به طور خودکار فعال می کند؟

خیر. SNI وارداتی هویت سرور واقعی است. برای فعال کردن این حالت، نام جعلی اختیاری جداگانه را تنظیم کنید.
قبل از فعال کردن [راه اندازی و محدودیت های SNI](https://github.com/Iman/caspian/wiki/SNI-Spoofing.fa) آن را بخوانید.

<a id="sni-idea-acknowledgements"></a>
## قدردانی ایده SNI

کاسپین همچنین به نویسندگان و مشارکت‌کنندگان این پروژه‌ها برای ایده‌ها و مقایسه‌های اجرایی که به کار SNI آن کمک می‌کند، اعتبار می‌دهد.
کد و فایل های اجرایی آن ها باندل نیستند.

- [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof): مقایسه ارسال SNI را انجام دهید.
- [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust): اجرای Rust SNI و مقایسه ویژگی ها.
- [bol-van/zapret](https://github.com/bol-van/zapret): استراتژی‌ها و تشخیص‌های دور زدن DPI.
- [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI): استراتژی های دور زدن DPI.
- [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android): ایده های یکپارچه سازی Android SNI.

کد تطبیقی مجوز و اطلاعیه های بالادستی خود را حفظ می کند.
تصدیق ایده اجازه کپی کد را نمی دهد یا به معنای تایید است.
برای نسخه های بازبینی شده، مجوزها و دامنه استفاده به [اعتبارات شخص ثالث](https://github.com/Iman/caspian/wiki/Third-Party-Credits.fa) مراجعه کنید.

[نتایج اعتبار سنجی و تست های سخت افزاری باقی مانده](https://github.com/Iman/caspian/blob/feature/sni/docs/SNI-VALIDATION.md).

</div>

<!-- English-source-sha256: 0c3a086c7127a1bec0deacf690b2aed7696a340ce94ec55379e1f233bee629f0 -->
