<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) | [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) | [中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh) | [العربية](https://github.com/Iman/caspian/wiki/Development-and-Testing.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Development-and-Testing.tr) | [اردو](https://github.com/Iman/caspian/wiki/Development-and-Testing.ur)

</div>

<div dir="rtl" align="right">

<a id="development-and-testing"></a>
# ترقی اور جانچ



[Caspian ویکی](https://github.com/Iman/caspian/wiki/Home.ur)

> یہ گائیڈ موجودہ README سے آتا ہے۔ اس کی پیمائش اپنی اصل تاریخوں کو برقرار رکھتی ہے۔ یہ دستاویزی اقدام نئے ٹیسٹ رن کی اطلاع نہیں دیتا ہے۔
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="running-it"></a>
## اسے چلانا

بائنری بنائیں اور اسے انسٹالر کے حوالے کریں۔ یہ راستہ کوئی رہائی کی ضرورت نہیں ہے، اور
انسٹالر اسے حقیقی انسٹال کے ساتھ ساتھ ڈرائی رن کے لیے بھی لیتا ہے:

go build -o /tmp/caspian-linux-arm64 ./cmd/caspian
sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
bash install.sh --dry-run --yes

اصل میں انسٹال کرنے کے لیے `--dry-run` ڈراپ کریں۔ `CASPIAN_LOCAL_CHECKSUMS` کے بغیر
انسٹالر ان الفاظ میں خبردار کرتا ہے کہ یہ ایک غیر تصدیق شدہ بائنری انسٹال کر رہا ہے۔
[`docs/INSTALL.md`](https://github.com/Iman/caspian/blob/main/docs/INSTALL.md) مکمل رن بک ہے۔ اس میں ایک جعلی `uname` ہارنس شامل ہے۔
انکار کو ایک مشین پر چلنا جس پر انسٹال نہیں کیا جاسکتا۔

بائنری میں چار ذیلی کمانڈز ہیں:

caspian serve --privileged root: راستے، فائر وال، ایکسیس پوائنٹ، انجن
caspian serve --panel the caspian user: ویب پینل، کچھ بھی مراعات یافتہ نہیں۔
Caspian چیک رپورٹ کریں کہ یہ باکس کیسا لگتا ہے۔ کچھ نہیں بدلتا
Caspian ورژن

جان بوجھ کر کوئی ذیلی کمانڈ نہیں ہے جو کنفگ کو لاگو کرتی ہے یا سوئچ کو چلاتی ہے۔
CLI خود ہی کہتا ہے: "انسٹالر کے چلنے کے بعد، ہر وہ کام جو ایک شخص کرتا ہے۔
پینل میں ہوتا ہے۔"

[`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) یونٹس، بائنری اور ڈائریکٹریز کو ہٹاتا ہے، اور دوبارہ چلاتا ہے
نیٹ ورک جرنل تو باکس کو اسی طرح چھوڑ دیا گیا جیسا کہ یہ پایا گیا تھا۔ اس پر بھروسہ کرنے سے پہلے [خرابی D5](https://github.com/Iman/caspian/wiki/Troubleshooting.ur) پڑھیں۔

<a id="the-rules-this-project-holds-itself-to"></a>
## اس منصوبے کے قواعد و ضوابط

یہ خواہشات نہیں ہیں۔ ہر ایک کا ایک طریقہ کار ہے، اور میکانزم کا نام ہے۔

**حقیقی ٹریفک سے کیپچر کیے گئے ایگزٹ آئی پی کے بغیر کام کرنے کو کچھ نہیں کہا جاتا۔**
[`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md)، سیکشن 6۔ رابطہ نتیجہ نہیں ہے۔ ہارڈ ویئر
ہارنس گریڈز غیر ثابت شدہ ہیں، پاس نہیں، جب کوئی ایگزٹ آئی پی کیپچر نہیں ہوا تھا، اور یہ 1 سے باہر نکلتا ہے۔

**ایک پُراعتماد غلط جملہ بغیر کسی جملے سے بدتر ہے۔** ایک قاری جسے بتایا جاتا ہے۔
کسی چیز کو صحیح طریقے سے سنبھالا جاتا ہے یہ نتیجہ اخذ کرتا ہے کہ چیک کرنے کے لئے کچھ نہیں ہے۔ تو اے
اصلاح ایک بہتر جملے کے بجائے ایک امتحان پیچھے چھوڑ دیتی ہے۔
`TestNothingInTheApplianceWatchesTheUplink` موجود ہے کیونکہ دو دستاویزات ایک بار
نے دعوی کیا کہ باکس اپنا اپلنک دیکھتا ہے اور جب حرکت کرتا ہے تو فائر وال کو دوبارہ لوڈ کرتا ہے۔

**شروع شدہ عمل اس بات کا ثبوت نہیں ہے کہ اس نے کام کیا۔** ہاٹ اسپاٹ انٹرفیس ہے۔
کسی بھی چیز سے منسلک ہونے سے پہلے دانا سے واپس پڑھیں، اور رسائی پوائنٹ ہے۔
سروس کے چلنے کی اطلاع دینے سے پہلے دوبارہ پڑھیں۔ دونوں ریڈ بیکس شامل کیے گئے تھے۔
ایک ناپے ہوئے واقعہ کے بعد جس میں ہر کمانڈ نے کامیابی حاصل کی تھی۔

**ہر منظر نامے کو ناکام ہوتے دیکھا گیا ہے۔** `TestEveryScenarioCanFail` ایک انجیکشن لگاتا ہے
ہر رویے میں عیب کا نام دیا گیا ہے اور اسے سرخ ہونے کی ضرورت ہے۔ ایسا امتحان کسی کا نہیں ہے۔
دیکھا ناکام ایک سبز روشنی ہے جو کچھ بھی نہیں ہے.

**کسی فکسچر کا اصل نام اس کے فائل نام میں ہے۔** `capture-pi5-` بائٹ ہے
ہدف پر ایک حقیقی کمانڈ کا آؤٹ پٹ، `scenario-` ایک مشین ہے جو کسی کے پاس نہیں ہے
ماپا گیا، اور `golden-` اس پروجیکٹ کا اپنا آؤٹ پٹ ہے۔ ایک امتحان پڑھنا a
`capture-pi5-` فائل ہدف کے بارے میں دعویٰ کرتی ہے۔ `scenario-` پڑھنے والا ٹیسٹ
فائل نہیں کرتا.

**کمٹ میں ایک سند مستقل ہوتی ہے۔** `test/goldenscan` ہر
رجسٹرڈ سینٹینلز اور اسناد کی شکلوں کے لیے کمٹڈ فکسچر، اور یہ
فائل کے ناموں کے ساتھ ساتھ فائل باڈیز کو بھی چیک کرتا ہے۔ یہ ایک پودے کو پکڑتے ہوئے دیکھا گیا ہے۔
ہر طبقے کا راز جسے وہ جانتا ہے۔

**کوریج کے فرش ایک شافٹ ہیں۔** [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh) میں ہر نمبر کیا ہے
کام کے بعد ماپا گیا ایک پیکیج جس نے اسے متعارف کرایا، کسی کو نشانہ بنانے کے لیے نہیں۔
کی امید تھی. ایک پیکیج جس میں کوئی قطار نہیں ہے گیٹ نہیں ہے، اور قطار کی عدم موجودگی کا مطلب ہے۔
"اس پیکیج کا احاطہ کیا گیا ہے" کے بجائے "ابھی تک کسی منزل پر اتفاق نہیں ہوا"۔

**مراعات یافتہ فریق کسی بھی چیز پر بھروسہ نہیں کرتا جو کال کرنے والا بھیجتا ہے۔** ہر ایک کا ہر شعبہ
درخواست کی جانچ پڑتال کی جاتی ہے کہ اس مشین نے خود کیا پتہ لگایا۔ انکار ہے a
بند سیٹ سے فالٹ کوڈ، کبھی کوئی جملہ نہیں، اور کبھی کال کرنے والے کی قدر نہیں۔
بھیجا

**باکس انٹرنیٹ سے ایسی کوئی چیز نہیں مانگتا جس کے لیے آپ نے اس سے نہیں پوچھا تھا۔** کوئی ٹیلی میٹری، کوئی فون ہوم، کوئی حادثہ
اپ لوڈ کریں، کوئی ویب فونٹ نہیں، کوئی جیو ڈیٹا فائل نہیں، اور کسی بھی ڈیفالٹ میں کوئی گوگل حل کرنے والا نہیں۔



[Architecture](https://github.com/Iman/caspian/wiki/Architecture.ur) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.ur) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting.ur)

<!-- Caspian guide navigation -->

Caspian گائیڈز: [سیٹ اپ اور معاون پروٹوکول](https://github.com/Iman/caspian/wiki/Home.ur) · [ڈی پی آئی کو روکنے کے لیے SNI کی جعل سازی: سیٹ اپ اور حدود](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ur)۔

</div>


<!-- English-source-sha256: 0b014c20f10040f03746de1a758beef6a154eef8fa3c308a7a8ca9f7b44707a3 -->
