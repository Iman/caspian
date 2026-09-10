<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) | [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) | [中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh) | [العربية](https://github.com/Iman/caspian/wiki/Development-and-Testing.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Development-and-Testing.tr) | [اردو](https://github.com/Iman/caspian/wiki/Development-and-Testing.ur)

</div>

<div dir="rtl" align="right">

<a id="development-and-testing"></a>
# توسعه و آزمایش



[ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa)

> این راهنما از README موجود می آید. اندازه گیری های آن تاریخ اصلی خود را حفظ می کند. این حرکت مستندسازی اجرای آزمایشی جدیدی را گزارش نمی‌کند.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="running-it"></a>
## اجرا کردنش

باینری را بسازید و به نصاب تحویل دهید. این مسیر نیازی به رهاسازی ندارد و
نصب کننده آن را برای نصب واقعی و همچنین اجرای خشک می گیرد:

برو build -o /tmp/caspian-linux-arm64 ./cmd/caspian
sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
bash install.sh --dry-run --yes

`--dry-run` را برای نصب واقعی رها کنید. بدون `CASPIAN_LOCAL_CHECKSUMS`
نصب کننده به این کلمات هشدار می دهد که در حال نصب یک باینری تایید نشده است.
[`docs/INSTALL.md`](https://github.com/Iman/caspian/blob/main/docs/INSTALL.md) یک runbook کامل است. این شامل یک مهار ساختگی `uname` برای
راه رفتن امتناع بر روی دستگاهی که قابل نصب نیست.

باینری چهار دستور فرعی دارد:

caspian serve -- root privileged: routes, firewall, access point, engine
caspian serve --panel the caspian user: پنل وب، هیچ چیز ممتازی ندارد
گزارش چک کاسپین که این کادر به چه شکل است. چیزی را تغییر نمی دهد
نسخه کاسپین

عمداً هیچ دستور فرعی وجود ندارد که پیکربندی را اعمال کند یا سوئیچ را هدایت کند.
خود CLI می‌گوید: «بعد از اجرای نصب‌کننده، هر کاری که شخص انجام می‌دهد
در پانل اتفاق می افتد."

[`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) واحدها، باینری و دایرکتوری ها را حذف می کند و دوباره پخش می کند.
مجله شبکه بنابراین جعبه همانطور که پیدا شد باقی می ماند. قبل از اینکه به آن اعتماد کنید، [نقص D5](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) را بخوانید.

<a id="the-rules-this-project-holds-itself-to"></a>
## قوانینی که این پروژه به آن پایبند است

اینها آرزو نیست. هر کدام مکانیسمی دارند و مکانیسم نامگذاری شده است.

**هیچ چیزی کار بدون IP خروجی گرفته شده از ترافیک واقعی نامیده نمی شود.**
[`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md)، بخش 6. اتصال یک نتیجه نیست. سخت افزار
هنگامی که IP خروجی ضبط نشده است، درجه های مهار UNPROVEN، نه PASS، و از 1 خارج می شود.

**جمله اشتباه مطمئن بدتر از بی جمله است.** خواننده ای که گفته می شود
چیزی که به درستی مدیریت می شود نتیجه می گیرد که چیزی برای بررسی وجود ندارد. بنابراین یک
تصحیح به جای یک جمله بهتر، یک آزمون را پشت سر می گذارد.
`TestNothingInTheApplianceWatchesTheUplink` وجود دارد زیرا دو سند یک بار
ادعا کرد که جعبه بالا لینک خود را تماشا می کند و هنگام حرکت فایروال را دوباره بارگذاری می کند.

**فرآیند شروع شده دلیلی بر کارکرد آن نیست.** رابط نقطه اتصال است
قبل از اینکه چیزی به هسته متصل شود، از هسته بازخوانی کنید، و نقطه دسترسی است
قبل از اجرای خود سرویس گزارش دهد. هر دو بازخوانی اضافه شد
پس از یک رویداد اندازه گیری شده که در آن هر فرمان موفقیت آمیز بود.

**هر سناریو شکست خورده است.** `TestEveryScenarioCanFail` یک
نقص در هر رفتار نامگذاری شده است و نیاز به قرمز شدن آن دارد. تستی که هیچکس ندارد
دیده می شود شکست یک چراغ سبز سیمی به هیچ است.

**منشا یک فیکسچر در نام فایل آن است.** `capture-pi5-` بایت است
خروجی یک فرمان واقعی روی هدف، `scenario-` ماشینی است که هیچ کس ندارد
اندازه گیری شد و `golden-` خروجی خود این پروژه است. یک تست خواندن الف
فایل `capture-pi5-` ادعایی در مورد هدف دارد. تست خواندن `scenario-`
فایل ندارد.

**اعتبار در یک commit دائمی است.**

`test/goldenscan`

هر جارو می کند
ثابت متعهد برای نگهبانان ثبت نام شده و برای اشکال اعتبار، و آن
نام فایل ها و همچنین بدنه فایل ها را بررسی می کند. در حال شکار یک کاشته تماشا شده است
راز هر طبقه ای که می داند

**طبقات پوشش یک جغجغه هستند.** هر عدد در [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh) همان چیزی است که
بسته ای که بعد از کاری که آن را معرفی کرده اندازه گیری می شود، نه یک فرد هدف
امیدوار شد. بسته‌ای که ردیفی ندارد دروازه‌بندی نمی‌شود و نبود ردیف به این معنی است
"هنوز طبقه ای توافق نشده است" به جای "این بسته پوشش داده شده است".

**طرف ممتاز به هیچ چیزی که تماس گیرنده ارسال می کند اعتماد ندارد.** هر فیلد از هر
درخواست با آنچه این دستگاه برای خود شناسایی کرده بررسی می شود. امتناع یک است
کد خطا از یک مجموعه بسته، هرگز یک جمله، و هرگز یک مقدار تماس گیرنده
فرستاده شد.

**جعبه از اینترنت چیزی نمی خواهد که شما از آن نخواسته اید.** بدون تله متری، بدون تلفن خانه، بدون خرابی
آپلود، بدون فونت وب، بدون فایل داده های جغرافیایی، و بدون حل کننده گوگل در هر پیش فرض.



[Architecture](https://github.com/Iman/caspian/wiki/Architecture.fa) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.fa) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting.fa)

<!-- Caspian guide navigation -->

راهنماهای کاسپین: [راه اندازی و پروتکل های پشتیبانی شده](https://github.com/Iman/caspian/wiki/Home.fa) · [جعل SNI برای دور زدن DPI: راه اندازی و محدودیت ها](https://github.com/Iman/caspian/wiki/SNI-Spoofing.fa).

</div>


<!-- English-source-sha256: 0b014c20f10040f03746de1a758beef6a154eef8fa3c308a7a8ca9f7b44707a3 -->
