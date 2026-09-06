# توسعه و آزمون

[English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) | [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) | [中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh)

[ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa)

> این راهنما از README موجود منتقل شده است. تاریخ اندازه‌گیری‌ها همان تاریخ اصلی است؛ این جابه‌جایی گزارش اجرای تازهٔ آزمون‌ها نیست.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## اجرای آن

فایل اجرایی را بسازید و به نصب‌کننده بدهید. این راه به هیچ انتشاری نیاز ندارد، و
نصب‌کننده آن را هم برای نصب واقعی و هم برای اجرای آزمایشی می‌پذیرد:

    go build -o /tmp/caspian-linux-arm64 ./cmd/caspian
    sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

    env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
        CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
        bash install.sh --dry-run --yes

`--dry-run` را بردارید تا نصبِ واقعی انجام شود. بدون `CASPIAN_LOCAL_CHECKSUMS`،
نصب‌کننده با همین واژه‌ها هشدار می‌دهد که دارد یک فایل اجراییِ وارسی‌نشده را نصب
می‌کند. [`docs/INSTALL.md`](https://github.com/Iman/caspian/blob/main/docs/INSTALL.md) دستورکار کامل است. یک بسترِ `uname` قلابی هم دارد تا
بشود ردکردن‌ها را روی دستگاهی که نصب روی آن ممکن نیست مرور کرد.

فایل اجرایی چهار زیرفرمان دارد:

    caspian serve --privileged     root: routes, firewall, access point, engine
    caspian serve --panel          the caspian user: the web panel, nothing privileged
    caspian check                  report what this box looks like; changes nothing
    caspian version

عمداً هیچ زیرفرمانی نیست که کانفیگی را اعمال کند یا کلید را بزند. خودِ CLI این را
می‌گوید: "After the installer has run, everything a person does happens in the
panel." یعنی: بعد از اجرای نصب‌کننده، هر کاری که آدم می‌کند در پنل انجام می‌شود.

[`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) یونیت‌ها، فایل اجرایی و پوشه‌ها را حذف می‌کند و دفترچهٔ شبکه را
بازپخش می‌کند تا دستگاه همان‌طور که پیدا شده رها شود. پیش از آنکه به آن تکیه
کنید، نقصِ D5 در پایین را بخوانید.

## قاعده‌هایی که این پروژه خودش را به آن‌ها پایبند می‌داند

این‌ها آرزو نیستند. هر کدام سازوکاری دارند، و آن سازوکار نام برده شده است.

**بدون گرفتنِ آدرسِ خروجی از ترافیکِ واقعی، هیچ چیز «کار می‌کند» نامیده
نمی‌شود.** [`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md)، بخش 6. یک اتصال، نتیجه نیست. بسترِ
سخت‌افزاری وقتی هیچ آدرسِ خروجی‌ای گرفته نشده باشد نمرهٔ UNPROVEN می‌دهد، نه
PASS، و با کد 1 بیرون می‌آید.

**یک جملهٔ غلطِ مطمئن از هیچ جمله‌ای بدتر است.** خواننده‌ای که به او گفته‌اند
چیزی درست رسیدگی شده، نتیجه می‌گیرد چیزی برای وارسی نیست. پس هر اصلاح، یک آزمون
از خودش به جا می‌گذارد، نه یک جملهٔ بهتر. `TestNothingInTheApplianceWatchesTheUplink`
به این دلیل وجود دارد که دو سند زمانی ادعا می‌کردند دستگاه رابطِ اینترنتش را
می‌پاید و وقتی جابه‌جا شود فایروال را دوباره بار می‌کند.

**یک فرایندِ شروع‌شده، شاهدِ کار کردنش نیست.** رابطِ هات‌اسپات پیش از آنکه چیزی
به آن بایند شود از هسته بازخوانی می‌شود، و نقطهٔ دسترسی پیش از آنکه سرویس خودش
را «در حال کار» گزارش کند بازخوانی می‌شود. هر دو بازخوانی بعد از یک رویدادِ
اندازه‌گیری‌شده اضافه شدند که در آن هر فرمانی موفقیت برگردانده بود.

**هر سناریو دیده شده که شکست بخورد.** `TestEveryScenarioCanFail` یک نقصِ نام‌دار
را به هر رفتار تزریق می‌کند و لازم می‌داند که قرمز شود. آزمونی که هیچ‌کس شکستش را
ندیده، چراغِ سبزی است که به هیچ چیز وصل نیست.

**تبارِ هر فیکسچر در نامِ فایلش است.** `capture-pi5-` خروجیِ بایتیِ یک فرمانِ
واقعی روی دستگاهِ هدف است، `scenario-` دستگاهی است که هیچ‌کس اندازه‌اش نگرفته، و
`golden-` خروجیِ خودِ این پروژه است. آزمونی که فایلِ `capture-pi5-` می‌خواند
دربارهٔ دستگاهِ هدف ادعا می‌کند. آزمونی که فایلِ `scenario-` می‌خواند چنین ادعایی
نمی‌کند.

**یک راز در یک کامیت، همیشگی است.** `test/goldenscan` هر فیکسچرِ کامیت‌شده را
برای نشانه‌های ثبت‌شده و برای شکل‌های رازها جارو می‌کند، و نامِ فایل‌ها را هم مثل
بدنهٔ فایل‌ها بررسی می‌کند. دیده شده که رازِ کاشته‌شده را از هر کلاسی که می‌شناسد
گرفته است.

**کف‌های پوشش فقط بالا می‌روند.** هر عددی در [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh) همان چیزی است که
یک بسته پس از کاری که آن را وارد کرد اندازه‌گیری شد، نه هدفی که کسی آرزویش را
داشت. بسته‌ای که سطری ندارد دروازه‌بندی نشده، و نبودِ سطر یعنی «هنوز کفی توافق
نشده»، نه «این بسته پوشش دارد».

**سمتِ ممتاز به هیچ چیزی که فراخواننده می‌فرستد اعتماد نمی‌کند.** هر فیلدِ هر
درخواست در برابرِ آنچه این دستگاه خودش تشخیص داده بررسی می‌شود. ردکردن یک کدِ خطا
از یک مجموعهٔ بسته است، هرگز یک جمله نیست، و هرگز مقداری که فراخواننده فرستاده
نیست.

**دستگاه از اینترنت چیزی نمی‌خواهد.** نه داده‌ای می‌فرستد، نه به خانه زنگ
می‌زند، نه گزارشِ خرابی بالا می‌فرستد، نه فونتِ وب می‌گیرد، نه فایلِ دادهٔ
جغرافیایی، و نه هیچ resolver گوگلی در هیچ پیش‌فرضی.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[Architecture](https://github.com/Iman/caspian/wiki/Architecture) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting)
