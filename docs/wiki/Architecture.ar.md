<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Architecture) | [فارسی](https://github.com/Iman/caspian/wiki/Architecture.fa) | [Русский](https://github.com/Iman/caspian/wiki/Architecture.ru) | [中文](https://github.com/Iman/caspian/wiki/Architecture.zh) | [العربية](https://github.com/Iman/caspian/wiki/Architecture.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Architecture.tr) | [اردو](https://github.com/Iman/caspian/wiki/Architecture.ur)

</div>

<div dir="rtl" align="right">

<a id="architecture-and-data-flow"></a>
# الهندسة المعمارية وتدفق البيانات



[ويكي قزوين](https://github.com/Iman/caspian/wiki/Home.ar)

> يأتي هذا الدليل من ملف README الموجود. تحتفظ قياساتها بتواريخها الأصلية. لا يُبلغ نقل التوثيق هذا عن تشغيل اختباري جديد.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="architecture"></a>
## الهندسة المعمارية

<a id="two-processes-one-binary"></a>
### عمليتان، واحدة ثنائية

يعمل ثنائي واحد في دورين، يتم اختيارهما بواسطة أمر فرعي. الانقسام موجود بحيث أ
لا يعد الخطأ في الجزء الذي يوزع إدخال المستخدم ويخدم HTTP خطأً في
الجزء الذي يحمل الجذر. [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md)، "عمليتان، واحدة ثنائية"، هي
بيان ثابت منه.

```mermaid
flowchart LR
    subgraph device["A device joined to the hotspot"]
        BR["Browser<br/>port 8088 on the hotspot address"]
    end

    subgraph panelproc["caspian serve --panel, runs as the caspian account"]
        PANEL["internal/panel<br/>routes, sessions, wording, rendering"]
        STATE["internal/state<br/>the only writer of state.json"]
        LINK1["internal/link<br/>parse the pasted share link"]
        ENG1["internal/engine<br/>Validate only, opens no socket"]
    end

    subgraph privproc["caspian serve --privileged, runs as root"]
        SVC["internal/privsvc<br/>Service.Start, Stop, Cut, Restore, Recover"]
        XCFG["internal/xcfg<br/>compose the engine document"]
        NETCFG["internal/netcfg<br/>routes, nftables, the teardown journal"]
        HOT["internal/hotspot<br/>hostapd and dnsmasq"]
        ENG2["internal/engine<br/>xray-core, in this process"]
    end

    BR --> PANEL
    PANEL --> STATE
    PANEL --> LINK1
    PANEL --> ENG1
    PANEL -->|"/run/caspian/priv.sock<br/>0660 root:caspian"| SVC
    SVC --> XCFG
    SVC --> NETCFG
    SVC --> HOT
    SVC --> ENG2
```

يطبع [`cmd/caspian/main.go`](https://github.com/Iman/caspian/blob/main/cmd/caspian/main.go) الدورين في نص الاستخدام الخاص به:

خدمة قزوين - الجذر المميز: الطرق، جدار الحماية، نقطة الوصول، المحرك
خدمة قزوين - لوحة مستخدم قزوين: لوحة الويب، لا يوجد شيء مميز

<a id="the-socket-and-why-the-vocabulary-is-closed"></a>
### المقبس، وسبب إغلاق المفردات

ينص [`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go) على القاعدة التي يوجد بها الانقسام بالكامل: "A
المساعد المميز الذي يأخذ المسار وقائمة الوسائط من عميله ليس كذلك
الحدود؛ إنها طريقة لتشغيل أي شيء كجذر." الفاصلة المنقوطة لهم. ال
يتم اقتباس الجملة بالضبط، لأن إعادة صياغة القاعدة ليست هي القاعدة.

لذلك لا يمكن للوحة التعبير عن "تشغيل هذا". ولا يمكن أن يسمي إلا إجراء واحدا من ثمانية أفعال،
والجانب المتميز هو الذي يقرر ما يعنيه كل واحد. `panel.Actions` هو ذلك
مجموعة مغلقة، ويفشل `TestActionVocabularyMatchesTheInterface` إذا كانت الطريقة كذلك
تمت إضافتها إلى الواجهة بدون اسم في القائمة.

| العمل | ما يفعله الجانب المميز | يغير الآلة |
|---|---|---|
| `detect` | قم بالإبلاغ عن الواجهات وحدود الراديو والشبكة الفرعية المختارة | لا |
| `status` | قم بالإبلاغ عن مرحلة المحرك ونقطة الاتصال وما إذا كانت حركة المرور مقطوعة | لا |
| `start` | قم بإحضار النفق ونقطة الاتصال للأعلى | نعم |
| `stop` | قم بإزالتهم وأعد تشغيل مجلة التفكيك | نعم |
| `recover` | توقف، أعد تشغيل المجلة، ثم ابدأ مرة أخرى من نفس الطلب | نعم |
| `engine-log` | قم بإرجاع الأسطر الأخيرة للمحرك، التي تم تنقيحها بالفعل | لا |
| `cut` | قم بإسقاط حركة مرور العميل المُعاد توجيهها واترك كل شيء آخر قيد التشغيل | نعم |
| `restore` | إعادة حركة مرور العميل المعاد توجيهها | نعم |

طلب واحد، استجابة واحدة، اتصال واحد. الرسالة عبارة عن نهاية كبيرة بحجم 4 بايت
length متبوعًا بالعديد من وحدات البايت من JSON. يتم التحقق من الطول
`maxFrameBytes` قبل تخصيص أي شيء أو تحليله، فهي رسالة كبيرة الحجم
تكاليف أربعة بايت والرفض. يتم رفض حقول JSON غير المعروفة بدلاً من ذلك
تم تجاهله. يتم فحص `protocolVersion` عند كل طلب. لذلك لوحة من واحد
إطلاق سراح التحدث إلى خدمة مميزة من شخص آخر يحصل على رفض محدد،
بدلاً من الحقل الذي تم فك تشفيره بصمت كقيمة صفرية.

لا شيء يعبر مرة أخرى على مسار الفشل باستثناء كلمة واحدة: `panel.Fault` من
مجموعة مغلقة، أو `privsvc.Refusal` من مجموعة مغلقة ثانية. المحرك الخاص
يتضمن نص الخطأ المادة الأساسية للمستخدم، لذلك يتم تسجيل دخوله إلى صاحب الامتياز
الجانب وانخفض. لا يوجد مجال للرد الذي يمكن أن يسافر فيه.

<a id="who-owns-which-package"></a>
### من يملك أي حزمة

```mermaid
flowchart TB
    LINK["internal/link<br/>share link in, one outbound out.<br/>Carries no credential in an exported field"]
    XCFG["internal/xcfg<br/>everything around the outbound:<br/>TUN inbound, SOCKS, local DNS, routing"]
    ENGINE["internal/engine<br/>starts and stops xray-core.<br/>Redacts every line on the way in"]
    NETCFG["internal/netcfg<br/>plans the machine, generates the ruleset,<br/>journals the inverse of every change"]
    HOTSPOT["internal/hotspot<br/>renders and supervises hostapd and dnsmasq.<br/>Detects no interface, queries no radio"]
    STATE["internal/state<br/>state.json, atomically, 0600"]
    PANEL["internal/panel<br/>the web interface and the fault vocabulary"]
    PRIVSVC["internal/privsvc<br/>the order of the steps, and the readbacks"]

    PANEL --> LINK
    PANEL --> STATE
    PRIVSVC --> LINK
    PRIVSVC --> XCFG
    PRIVSVC --> NETCFG
    PRIVSVC --> HOTSPOT
    PRIVSVC --> ENGINE
    LINK --> XCFG
    XCFG --> ENGINE
```

`internal/privsvc` يعيد توزيع `StartRequest.ConfigJSON` مع `internal/link`
بدلاً من الثقة في قيام اللجنة بذلك. كما أنه يتحقق من الإنترنت
واجهة مقابل المسار الافتراضي لهذا الجهاز، واجهة نقطة الاتصال
مقابل مخرج `iw list` الخاص بهذا الجهاز، والقناة مقابل ما
ذكرت الراديو أنها قابلة للاستخدام.

<a id="where-state-lives-and-who-writes-it"></a>
### أين تعيش الدولة ومن يكتبها

كاتبان، ملفان، لا يوجد ملف مشترك. لا تكتب أي من العمليتين عملية الأخرى، لذلك
لا يوجد قفل ولا يوجد تحديث مفقود للحماية منه. [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md)، "من
يكتب ماذا"، يسجل القرار والمسودة السابقة التي نقضها.

```mermaid
flowchart TB
    subgraph panelowns["Written only by caspian serve --panel"]
        SJ["/var/lib/caspian/state.json<br/>0600 caspian. Holds the pasted config<br/>and the hotspot passphrase"]
    end

    subgraph privowns["Written only by caspian serve --privileged"]
        JN["/var/lib/caspian/netcfg.journal<br/>0600 root. The inverse of every change,<br/>written before the change"]
        HC["/run/caspian/hostapd.conf<br/>0600 root, tmpfs, rewritten every start"]
        DC["/run/caspian/dnsmasq.conf<br/>0600 root, tmpfs, rewritten every start"]
    end

    subgraph nofile["Held in memory and written to no file"]
        CUT["the cut"]
        EVT["the panel's event list"]
        RING["the engine log ring"]
    end
```

لا يقرأ الجانب المميز أي ملف حالة على الإطلاق. كل ما يحتاجه يصل
طلب البداية. يقوم `TestPrivsvcReadsNoStateFile` بفحص المصدر الخاص بتلك الحزمة
ويفشل إذا قرأ واحدة على الإطلاق، وهو ما لم يكن ليقدمه تعليق.

الجدول الكامل للمسارات والأوضاع والمالكين موجود في [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md). الموانئ هي
تم إصلاحه هناك أيضًا: 53 للعميل DNS على نقطة الاتصال، 5354 عند الاسترجاع لـ
مستمع DNS الخاص بالمحرك، 8088 للوحة، 10808 عند الاسترجاع لـ
الجوارب التشخيصية واردة.

<a id="how-data-flows"></a>
## كيف تتدفق البيانات

<a id="a-pasted-share-link-becomes-a-running-tunnel"></a>
### يصبح رابط المشاركة الذي تم لصقه نفقًا قيد التشغيل

`startNow` في [`internal/panel/handlers.go`](https://github.com/Iman/caspian/blob/main/internal/panel/handlers.go) يوثق الأمر، والأمر هو
ما الذي يفصل بين فشل التكوين الثلاثة. لم يتم لمس أي شيء على الجهاز
حتى تنتهي الحالة 1 والحالة 2.

```mermaid
sequenceDiagram
    autonumber
    participant U as The person at the panel
    participant PA as internal/panel
    participant LK as internal/link
    participant EN as internal/engine
    participant PS as internal/privsvc, root
    participant NC as internal/netcfg
    participant HS as internal/hotspot

    U->>PA: POST /power, on=1
    PA->>LK: link.Parse of the stored text
    Note over LK: State 1. It did not parse.<br/>The user has to fix the text.
    LK-->>PA: a Link that holds no credential in any exported field
    PA->>LK: Link.XrayConfig
    LK-->>PA: one outbound, tagged proxy, nulls removed
    PA->>EN: engine.Validate
    Note over EN: State 2. Read, and unusable as written.<br/>No socket opens. Nothing is dialled.
    PA->>PS: StartRequest over priv.sock
    PS->>PS: clock floor, re-parse, validate against this machine
    PS->>NC: Detect, then PlanNetwork
    PS->>PS: xcfg.Build, then engine.Validate again
    PS->>NC: Apply PreEngineSteps. The firewall is first.
    PS->>NC: AssertHotspotInterfaceReleased
    PS->>EN: Engine.Start. The tunnel device appears here.
    PS->>NC: Apply PostEngineSteps. Each needs the tunnel or engine listener.
    PS->>HS: Supervisor.Start: hostapd, then dnsmasq
    PS->>NC: AssertHotspotIsAccessPoint
    PS->>PS: probe the server
    Note over PS: State 3. The link was fine and the<br/>server did not answer. No rollback:<br/>the box is fully configured and blocking.
    PS-->>PA: nil, or one panel.Fault
```

ثلاثة تفاصيل في هذا التسلسل هي الحاملة.

تم إنشاء مستند المحرك مرتين لأسباب مختلفة. `internal/link`
ينتج الخارج ولا شيء غير ذلك. `internal/xcfg` تنتج كل شيء
من حوله: شبكة TUN الواردة التي تصل إليها حركة مرور العميل، وSOCKS الاسترجاع
الوارد الذي تستخدمه التشخيصات والوكيل المؤقت لنظام macOS، DNS المحلي
المستمع وسياسة المحلل وقواعد التوجيه.
ولا يؤخذ شيء من ذلك مما أرسله المتصل.

البداية التي تفشل جزئيًا يتم التراجع عنها تمامًا. المجلة بالفعل
يحمل معكوس كل تغيير، مكتوبًا على القرص قبل أن يصل التغيير إلى
نواة. البداية التي تفشل تترك الآلة كما تم العثور عليها.

الخادم الذي لا يجيب ليس مربعًا نصف مطبق. كل تغيير نجح
جدار الحماية ساري المفعول، ويتم حظر حركة مرور العميل المعاد توجيهها بسبب
النفق لا يحمل شيئا فيبلغ العيب ولا يهدم شيء.

<a id="the-network-path-of-a-client-packet"></a>
### مسار الشبكة لحزمة العميل

```mermaid
flowchart TB
    DEV["A joined device<br/>address from dnsmasq"] --> IF["The hotspot interface"]
    IF --> PRE["nft chain prerouting, type nat<br/>DNS on port 53 is redirected here"]
    PRE --> ROUTE{"Routing decision<br/>ip rule from the hotspot subnet<br/>lookup table 8410"}
    ROUTE -->|"tunnel route present"| TOTUN["oif is the tunnel device<br/>default route in table 8410"]
    ROUTE -->|"tunnel route withdrawn"| TOUP["oif is the uplink"]
    TOTUN --> FW1["nft chain forward, policy drop"]
    TOUP --> FW2["nft chain forward, policy drop"]
    FW1 -->|"iifname hotspot oifname tunnel<br/>ip saddr the hotspot subnet, accept"| POST["nft chain postrouting<br/>deliberately empty, no masquerade"]
    FW2 -->|"iifname hotspot oifname uplink, drop<br/>the leak block, first rule in the chain"| DROP["dropped"]
    POST --> TUN["The tunnel device<br/>a userspace netstack in the engine"]
    TUN --> OB["the outbound tagged proxy"]
    OB --> UP["The uplink<br/>a pinned host route to the server"]
    UP --> SRV["Your server"]
```

تقوم كتلة التسرب بتسمية نقطة الاتصال والوصلة الصاعدة فقط. لا يمكن أن يتوقف عن العمل
عندما يذهب النفق، لأنه لم يذكر النفق. كل حكم ذلك
تسمح حركة مرور العميل بتسمية النفق، لذلك تتوقف هذه القواعد عن المطابقة و
السياسة تسقط كل شيء.

تتم مطابقة كل واجهة بالاسم وليس بالفهرس أبدًا. يتم حل الفهرس عندما
يتم تحميل مجموعة القواعد، لذلك لا يمكن تحميل مجموعة القواعد التي تسمي النفق حسب الفهرس أثناء تشغيل
النفق معطل، وهو بالضبط الوقت الذي يجب أن يدخل فيه حيز التنفيذ.

سلسلة ما بعد التوجيه فارغة عن قصد. حفلة تنكرية تجاه الوصلة الصاعدة هي
الخط الوحيد الذي من شأنه أن يحول الجهاز بهدوء إلى جهاز توجيه عادي.

<a id="what-the-tunnel-disappearing-does-to-that-path"></a>
### ما يفعله اختفاء النفق بهذا المسار

```mermaid
flowchart TB
    GONE["The tunnel stops carrying traffic"] --> Q{"Does the device still exist?"}
    Q -->|"device removed"| WD["The kernel withdraws every route through it"]
    WD --> FB["Client traffic falls back to the main table<br/>and heads for the uplink"]
    FB --> LB["The leak block matches: iifname hotspot oifname uplink, drop"]
    Q -->|"device persists with nothing servicing it"| ENTER["Traffic enters the tunnel device"]
    ENTER --> NOWHERE["Nothing reads it. It goes no further."]
    LB --> SAFE["No client traffic leaves"]
    NOWHERE --> SAFE
```

الفرع الذي يحدث لم يتم تسويته. [`internal/netcfg/testdata/PROVENANCE.md`](https://github.com/Iman/caspian/blob/main/internal/netcfg/testdata/PROVENANCE.md)
يسجل ملاحظة من الهدف بتاريخ 30-08-2026: `xray0` كان موجودا في
تم إيقاف تشغيل قائمة أجهزة NetworkManager مع الخدمة، كما
`connected (externally)`. لا شيء هنا يحدد السبب، والمحرك ليس كذلك
رمز هذا المشروع. لا يتسرب أي من الفرعين، ولا يعتمد أي منهما على معرفة أي منهما
يحدث واحد. ولهذا السبب تمت كتابة الكتلة لتسمية نقطة الاتصال ونقطة الاتصال فقط
الوصلة الصاعدة.

<a id="the-dns-path-which-is-not-the-traffic-path"></a>
### مسار DNS، وهو ليس مسار حركة المرور

هذا هو الجزء الذي يخطئ فيه الناس. سؤال DNS الخاص بالعميل ليس مجرد سؤال
مسموح به. يؤخذ.

```mermaid
flowchart TB
    ASK["A joined device asks whatever resolver it was told to use,<br/>or one hardcoded into it, on port 53"]
    ASK --> RD["nft prerouting on the hotspot:<br/>udp dport 53 and tcp dport 53 redirect to :53<br/>The destination address is rewritten to this box"]
    RD --> DM["dnsmasq, bound to the hotspot interface<br/>/run/caspian/dnsmasq.conf"]
    DM -->|"its only permitted upstream is a loopback address"| LD["the engine's DNS listener<br/>127.0.0.1:5354, inbound tag local-dns-in"]
    LD --> R1["rule ruleTagLocalDNS<br/>inboundTag local-dns-in, outbound dns-out"]
    R1 --> APP["the engine's DNS app<br/>resolvers from internal/xcfg/resolvers.go"]
    APP --> R2["rule ruleTagResolvers<br/>inboundTag resolver-in, outbound proxy.<br/>Above the private-address rule"]
    R2 --> OB["the outbound tagged proxy"]
    OB --> EXIT["the resolver chain, reached from the far end of the tunnel"]
```

وهذه السلسلة أربع خواص، كل منها بما يحملها.

تقوم عملية إعادة التوجيه بإعادة كتابة الوجهة، بحيث يكون الجهاز مزودًا بوحدة تحليل مضمنة
يتم الرد عليه هنا بدلاً من السماح له بالوصول إلى الشخص الذي قيل له
استخدام. السيناريو: "لا يمكن للعميل الوصول إلى محلل من اختياره".

يقوم عرض DHCP بتسمية هذا المربع مرة واحدة وليس أي محلل آخر. وهذا يستحق خاصة به
السيناريو لأن الخطأ هو أمر غير مرئي: ستعيد عملية إعادة التوجيه كتابة ملف
الحزم على أي حال، لذلك لن يبدو أي شيء على السلك خاطئًا. السيناريو: "الصندوق
تقدم نفسها كمحلل ولا تسمي شخصًا آخر أبدًا".

يرفض `internal/hotspot` أي منبع dnsmasq ليس عنوان استرجاع.
سيكون الهدف غير القابل للاسترجاع عبارة عن استعلام يترك الصندوق خارج النفق
كل اسم يطلبه كل عميل. مستمع المحرك هو ما يجيب هناك،
ويفشل `TestLocalDNSDefaultMatchesTheHotspotUpstream` إذا انحرف المنفذان.
[`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md) يستدعي هذا الاقتران الذي ينكسر بهدوء: إذا كان الاثنان
الانجراف، يتوقف كل جهاز متصل عن العمل بينما تكون نقطة الاتصال والنفق معًا
تبدو صحية.

القاعدة التي ترسل استعلامات المحلل الخاصة إلى النفق تقع فوق
القاعدة التي ترسل عناوين خاصة مباشرة. لذلك محلل على عنوان خاص
لا يزال يتم الوصول إليها عبر النفق وليس على الشبكة المحلية.
`TestLocalDNSQueriesCannotFallOutToTheUplink` و`TestPrivateRangesRouteDirect`
عقد النصفين.

سلسلة المحلل نفسها عبارة عن ثلاثة مشغلين في ثلاث ولايات قضائية: Quad9's
الخدمة التي تمت تصفيتها ومتغير Cloudflare FAMILY وأمن CleanBrowsing.
يسجل [`internal/xcfg/resolvers.go`](https://github.com/Iman/caspian/blob/main/internal/xcfg/resolvers.go) سبب كل منها، وأي منها متطابق تقريبًا
عنوان نفس المشغل عمدا لا. لا يظهر أي محلل جوجل
في أي افتراضي، و`TestNoGoogleAnywhereInGeneratedConfigs` بمسح كل
الوثيقة التي تم إنشاؤها لأحد.

يتم التعامل مع المنافذ الأخرى ولا يمكن أن يكون أحدها:

```mermaid
flowchart LR
    DOT["DNS over TLS<br/>tcp 853"] --> REJ["reject with tcp reset,<br/>so the device falls back to port 53"]
    DOQ["DNS over QUIC<br/>udp 853"] --> DRP["drop"]
    DOH["DNS over HTTPS<br/>port 443"] --> CAR["carried through the tunnel like any HTTPS.<br/>Not a leak. Not visible to anything here."]
```



<!-- Caspian guide navigation -->

أدلة Caspian: [الإعداد والبروتوكولات المدعومة](https://github.com/Iman/caspian/wiki/Home.ar) · [انتحال SNI للتحايل على DPI: الإعداد والحدود](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ar).

</div>


<!-- English-source-sha256: 07a2e0584db74a162eb7938428d733054b00788748889213f96e0e0679d0f650 -->
