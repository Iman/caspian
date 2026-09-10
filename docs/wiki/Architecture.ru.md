<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Architecture) · [فارسی](https://github.com/Iman/caspian/wiki/Architecture.fa) · [**Русский**](https://github.com/Iman/caspian/wiki/Architecture.ru) · [简体中文](https://github.com/Iman/caspian/wiki/Architecture.zh) · [العربية](https://github.com/Iman/caspian/wiki/Architecture.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/Architecture.tr) · [اردو](https://github.com/Iman/caspian/wiki/Architecture.ur)

</div>

<div dir="ltr" lang="ru">

[Вики Caspian](https://github.com/Iman/caspian/wiki/Home.ru) · [Устранение неполадок](https://github.com/Iman/caspian/wiki/Troubleshooting.ru)

</div>
<!-- wiki-navigation:end -->

<a id="architecture-and-data-flow"></a>
# Архитектура и поток данных

> Это руководство взято из существующего README. Его измерения сохраняют свои первоначальные даты; этот шаг документации не сообщает о новом тестовом запуске.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="architecture"></a>
## Архитектура

<a id="two-processes-one-binary"></a>
### Два процесса, один двоичный файл

Один двоичный файл выполняется в двух ролях, выбираемых подкомандой. Раскол существует так, что
ошибка в части, которая анализирует пользовательский ввод и обслуживает HTTP, не является ошибкой в
часть, удерживающая корень. [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md), «Два процесса, один двоичный файл», — это
фиксированное заявление об этом.

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

[`cmd/caspian/main.go`](https://github.com/Iman/caspian/blob/main/cmd/caspian/main.go) печатает две роли в своем собственном тексте использования:

каспийская служба --привилегированный корень: маршруты, брандмауэр, точка доступа, движок
каспийский сервис --panel пользователь каспийского: веб-панель, без привилегий

<a id="the-socket-and-why-the-vocabulary-is-closed"></a>
### Розетка, и почему словарь закрыт

[`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go) устанавливает правило, по которому существует все разделение: «A
привилегированный помощник, который принимает путь и список аргументов от своего клиента, не является
граница; это способ запустить что-либо с правами root». Точка с запятой принадлежит им.
предложение цитируется точно, потому что перефразирование правила не является правилом.

Таким образом, комиссия не может сказать «запустите это». Он может назвать только одно из восьми действий,
и привилегированная сторона решает, что означает каждый из них. `panel.Actions` это
закрытый набор, и `TestActionVocabularyMatchesTheInterface` завершается с ошибкой, если метод
добавлен в интерфейс без имени в списке.

| Действие | Что делает привилегированная сторона | Меняет машину |
|---|---|---|
| `detect` | Сообщите об интерфейсах, ограничениях радиосвязи и выбранной подсети. | нет |
| `status` | Сообщайте о фазе двигателя, точке доступа и о том, отключен ли трафик. | нет |
| `start` | Поднимите туннель и точку доступа. | да |
| `stop` | Уничтожьте их и просмотрите журнал разборки. | да |
| `recover` | Остановитесь, воспроизведите журнал, затем начните снова с того же запроса. | да |
| `engine-log` | Вернуть последние строки движка, уже отредактированные. | нет |
| `cut` | Отбросьте перенаправленный клиентский трафик и оставьте все остальное включенным | да |
| `restore` | Вернуть перенаправленный клиентский трафик | да |

Один запрос, один ответ, одно соединение. Сообщение представляет собой 4-байтовый код с обратным порядком байтов.
длина, за которой следует такое же количество байтов JSON. Длина сверяется с
`maxFrameBytes` перед тем, как что-либо будет выделено или проанализировано, поэтому сообщение слишком большого размера.
стоит четыре байта и отказ. Неизвестные поля JSON отклоняются, а не
игнорируется. `protocolVersion` проверяется при каждом запросе. Итак, панель из одного
освободить разговор с привилегированным сервисом от другого, получив поименованный отказ,
вместо поля, молча декодируемого как его нулевое значение.

Ничто не возвращается на путь отказа, кроме одного слова: `panel.Fault` от
закрытый набор или `privsvc.Refusal` из второго закрытого набора. Двигатель собственный
текст ошибки включает в себя ключевой материал пользователя, поэтому он регистрируется в привилегированном
в сторону и упал. В ответе нет поля, в котором он мог бы пройти.

<a id="who-owns-which-package"></a>
### Кому какой пакет принадлежит

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

`internal/privsvc` повторно анализирует `StartRequest.ConfigJSON` с помощью `internal/link`
вместо того, чтобы доверять комиссии, чтобы сделать это. Он также проверяет Интернет
интерфейс против собственного маршрута по умолчанию этого компьютера, интерфейс точки доступа
против собственного выхода `iw list` этой машины, а канал против того, что
Радио сообщается как пригодное к использованию.

<a id="where-state-lives-and-who-writes-it"></a>
### Где живет государство и кто его пишет

Два автора, два файла, нет общего файла. Ни один из процессов не записывает данные другого, поэтому
нет блокировки и потери обновлений, от которых можно было бы защититься. [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md), "Кто
что пишет", записывает решение и более ранний проект, который он отменил.

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

Привилегированная сторона вообще не читает файл состояния. Все, что ему нужно, поступает
запрос на запуск. `TestPrivsvcReadsNoStateFile` сканирует собственный источник этого пакета.
и терпит неудачу, если он когда-либо прочитает тот, который не был бы предоставлен в комментарии.

Полная таблица путей, режимов и владельцев находится в [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md). Порты
там тоже исправлено: 53 для клиентского DNS в точке доступа, 5354 в шлейфе для
прослушиватель DNS движка, 8088 для панели, 10808 на шлейфе для
диагностика SOCKS входящая.

<a id="how-data-flows"></a>
## Как передаются данные

<a id="a-pasted-share-link-becomes-a-running-tunnel"></a>
### Вставленная ссылка общего доступа становится работающим туннелем.

`startNow` в [`internal/panel/handlers.go`](https://github.com/Iman/caspian/blob/main/internal/panel/handlers.go) документирует заказ, и заказ
что отличает три ошибки конфигурации друг от друга. В машине ничего не трогается
пока состояние 1 и состояние 2 не пройдут.

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

Три детали в этой последовательности являются несущими.

Документ двигателя составляется дважды по разным причинам. `internal/link`
производит исходящий трафик и ничего больше. `internal/xcfg` производит все
вокруг него: входящий TUN, на который поступает клиентский трафик, шлейф SOCKS
входящие, используемые диагностикой и временным системным прокси-сервером macOS, локальным DNS
прослушиватель, политика преобразователя и правила маршрутизации.
Ничего из этого не взято из того, что отправил вызывающий абонент.

Старт, который провалился на полпути, полностью отменяется. Журнал уже
хранит инверсию каждого изменения, записанную на диск до того, как изменение достигло
ядро. При неудачном запуске машина остается в том виде, в котором она была обнаружена.

Сервер, который не отвечает, это не полуприложенный ящик. Каждое изменение удалось,
брандмауэр действует, и пересылаемый клиентский трафик блокируется, поскольку
туннель ничего не несет. Таким образом, о неисправности сообщается, и ничего не сносится.

<a id="the-network-path-of-a-client-packet"></a>
### Сетевой путь клиентского пакета

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

Блок утечки называет только точку доступа и восходящий канал. Он не может перестать работать
когда туннель идет, потому что там не упоминается туннель. Каждое правило, которое
разрешает клиентский трафик, но называет туннель, поэтому эти правила перестают совпадать и
политика отбрасывает все.

Каждый интерфейс сопоставляется по имени, а не по индексу. Индекс разрешается, когда
набор правил загружается, поэтому набор правил, именующий туннель по индексу, не может загружаться, пока
туннель не работает, а это именно тот момент, когда он должен действовать.

Цепочка postrouting намеренно пуста. Маскарад в сторону восходящей линии связи
единственная линия, которая незаметно превратила бы устройство в обычный маршрутизатор.

<a id="what-the-tunnel-disappearing-does-to-that-path"></a>
### Что исчезновение туннеля делает с этим путем

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

Какая именно ветка происходит, не установлено. [`internal/netcfg/testdata/PROVENANCE.md`](https://github.com/Iman/caspian/blob/main/internal/netcfg/testdata/PROVENANCE.md)
записывает наблюдение с цели 30 августа 2026 г.: `xray0` присутствовал в
Список устройств NetworkManager с отключенной службой, т.к.
`connected (externally)`. Здесь ничего не установлено, почему и двигатель не работает.
код этого проекта. Ни одна из ветвей не дает утечек, и ни одна из них не зависит от знания того, какая
случается одно. Вот почему блок был написан так, чтобы называть только хотспот и
восходящая линия связи.

<a id="the-dns-path-which-is-not-the-traffic-path"></a>
### Путь DNS, который не является путем трафика.

Это то, в чем люди ошибаются. Вопрос DNS клиента – это не просто
разрешено. Оно взято.

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

Четыре свойства этой цепочки, каждое из которых имеет предмет, который его удерживает.

Перенаправление перезаписывает пункт назначения, поэтому устройство с жестко запрограммированным преобразователем
здесь дается ответ, а не позволяется добраться до того, кому было сказано
использовать. Сценарий: «клиент не может связаться с резолвером по своему выбору».

Предложение DHCP называет это поле один раз, а не другой преобразователь. Это того стоит
сценарий, потому что ошибка невидима: перенаправление перепишет
пакеты в любом случае, поэтому ничто в проводе не будет выглядеть неправильно. Сценарий: «коробка
предлагает себя в качестве решателя и никогда не называет другого».

`internal/hotspot` отклоняет любой восходящий поток dnsmasq, который не является адресом обратной связи.
Целью без шлейфа может быть запрос, выходящий за пределы туннеля, поскольку
каждое имя, которое просит каждый клиент. Слушатель двигателя - это то, что там отвечает,
и `TestLocalDNSDefaultMatchesTheHotspotUpstream` дает сбой, если два порта смещаются.
[`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md) называет эту пару той, которая распадается тихо: если два
дрейф, каждое подключенное устройство перестает разрешать, в то время как точка доступа и туннель оба
выглядеть здоровым.

Правило, которое отправляет собственные запросы преобразователя в туннель, находится над правилом
правило, которое отправляет частные адреса напрямую. Таким образом, преобразователь на частном адресе
по-прежнему достигается через туннель, а не по локальной сети.
`TestLocalDNSQueriesCannotFallOutToTheUplink` и `TestPrivateRangesRouteDirect`
держите две половинки.

Сама цепочка резольверов состоит из трех операторов в трех юрисдикциях: Quad9
фильтрованный сервис, вариант Cloudflare FAMILY и CleanBrowsing Security.
[`internal/xcfg/resolvers.go`](https://github.com/Iman/caspian/blob/main/internal/xcfg/resolvers.go) записывает, почему каждый из них и какой из них практически идентичен.
адрес того же оператора намеренно не указан. Резолвер Google не отображается
по умолчанию, а `TestNoGoogleAnywhereInGeneratedConfigs` сканирует каждый
созданный документ для одного.

Остальные порты обрабатываются, и один из них не может быть:

```mermaid
flowchart LR
    DOT["DNS over TLS<br/>tcp 853"] --> REJ["reject with tcp reset,<br/>so the device falls back to port 53"]
    DOQ["DNS over QUIC<br/>udp 853"] --> DRP["drop"]
    DOH["DNS over HTTPS<br/>port 443"] --> CAR["carried through the tunnel like any HTTPS.<br/>Not a leak. Not visible to anything here."]
```

<!-- English-source-sha256: c61eb46a98fa09164d26fd64714b16205c6a95bd9f2b26e13ccca199283bcb0d -->
