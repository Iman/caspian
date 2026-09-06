# Архитектура и потоки данных

[English](https://github.com/Iman/caspian/wiki/Architecture) | [فارسی](https://github.com/Iman/caspian/wiki/Architecture.fa) | [Русский](https://github.com/Iman/caspian/wiki/Architecture.ru) | [中文](https://github.com/Iman/caspian/wiki/Architecture.zh)

[Вики Caspian](https://github.com/Iman/caspian/wiki/Home.ru)

> Руководство перенесено из README. Даты измерений сохранены; перенос документации не означает нового запуска тестов.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## Архитектура

### Два процесса, один бинарник

Один бинарник работает в двух ролях, и роль выбирается подкомандой. Разделение
существует для того, чтобы сбой в той части, которая разбирает пользовательский
ввод и обслуживает HTTP, не был сбоем в той части, которая держит root.
[`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md), раздел «Two processes, one binary», является закреплённой
формулировкой этого.

```mermaid
flowchart LR
    subgraph device["Устройство, подключённое к хотспоту"]
        BR["Браузер<br/>порт 8088 на адресе хотспота"]
    end

    subgraph panelproc["caspian serve --panel, работает под учётной записью caspian"]
        PANEL["internal/panel<br/>маршруты, сессии, тексты, рендеринг"]
        STATE["internal/state<br/>единственный, кто пишет state.json"]
        LINK1["internal/link<br/>разбор вставленной share-ссылки"]
        ENG1["internal/engine<br/>только Validate, не открывает сокетов"]
    end

    subgraph privproc["caspian serve --privileged, работает под root"]
        SVC["internal/privsvc<br/>Service.Start, Stop, Cut, Restore, Recover"]
        XCFG["internal/xcfg<br/>сборка документа для движка"]
        NETCFG["internal/netcfg<br/>маршруты, nftables, журнал разборки"]
        HOT["internal/hotspot<br/>hostapd и dnsmasq"]
        ENG2["internal/engine<br/>xray-core, в этом же процессе"]
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

[`cmd/caspian/main.go`](https://github.com/Iman/caspian/blob/main/cmd/caspian/main.go) печатает обе роли в собственной справке:

    caspian serve --privileged     root: routes, firewall, access point, engine
    caspian serve --panel          the caspian user: the web panel, nothing privileged

### Сокет и почему его словарь закрыт

[`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go) формулирует правило, ради которого и существует всё это
разделение: "A privileged helper that takes a path and an argument list from its
client is not a boundary; it is a way to run anything as root." То есть:
привилегированный помощник, принимающий от своего клиента путь и список
аргументов, не является границей; это способ запустить что угодно от имени root.
Точка с запятой их. Фраза процитирована дословно, потому что пересказ правила не
является правилом.

Поэтому панель не может выразить «выполни это». Она может только назвать одно из
восьми действий, а привилегированная сторона решает, что каждое из них значит.
`panel.Actions` и есть это закрытое множество, а
`TestActionVocabularyMatchesTheInterface` падает, если в интерфейс добавлен
метод, которому нет имени в списке.

| Действие | Что делает привилегированная сторона | Меняет машину |
|---|---|---|
| `detect` | Сообщает интерфейсы, ограничения радио и выбранную подсеть | нет |
| `status` | Сообщает фазу движка, состояние хотспота и то, отсечён ли трафик | нет |
| `start` | Поднимает туннель и хотспот | да |
| `stop` | Опускает их и проигрывает журнал разборки | да |
| `recover` | Останавливает, проигрывает журнал, затем запускает заново из того же запроса | да |
| `engine-log` | Возвращает последние строки лога движка, уже вычищенные | нет |
| `cut` | Обрывает пересылаемый клиентский трафик, оставляя всё остальное работать | да |
| `restore` | Возвращает пересылаемый клиентский трафик обратно | да |

Один запрос, один ответ, одно соединение. Сообщение, это 4-байтовая длина в
порядке big-endian, за которой следует столько же байтов JSON. Длина
проверяется против `maxFrameBytes` до того, как что-либо будет выделено или
разобрано, поэтому слишком большое сообщение стоит четырёх байтов и отказа.
Неизвестные поля JSON отклоняются, а не игнорируются. `protocolVersion`
проверяется в каждом запросе. Поэтому панель из одного релиза, говорящая с
привилегированной службой из другого, получает поименованный отказ вместо поля,
молча разобранного как нулевое значение.

По пути ошибки обратно не переходит ничего, кроме одного слова: `panel.Fault` из
закрытого множества или `privsvc.Refusal` из второго закрытого множества. В
собственном тексте ошибок движка встроен ключевой материал пользователя, поэтому
он записывается в лог на привилегированной стороне и отбрасывается. В ответе нет
поля, в котором он мог бы уехать.

### Какой пакет за что отвечает

```mermaid
flowchart TB
    LINK["internal/link<br/>на входе share-ссылка, на выходе один outbound.<br/>Не несёт учётных данных ни в одном экспортированном поле"]
    XCFG["internal/xcfg<br/>всё вокруг outbound:<br/>входящий TUN, SOCKS, локальный DNS, маршрутизация"]
    ENGINE["internal/engine<br/>запускает и останавливает xray-core.<br/>Вычищает каждую строку на входе"]
    NETCFG["internal/netcfg<br/>планирует машину, генерирует набор правил,<br/>журналирует обратное действие для каждого изменения"]
    HOTSPOT["internal/hotspot<br/>рендерит и надзирает за hostapd и dnsmasq.<br/>Не определяет интерфейсы, не опрашивает радио"]
    STATE["internal/state<br/>state.json, атомарно, 0600"]
    PANEL["internal/panel<br/>веб-интерфейс и словарь ошибок"]
    PRIVSVC["internal/privsvc<br/>порядок шагов и обратные считывания"]

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

`internal/privsvc` заново разбирает `StartRequest.ConfigJSON` через
`internal/link`, а не доверяет тому, что это уже сделала панель. Он также
сверяет интерфейс выхода в интернет с собственным маршрутом по умолчанию этой
машины, интерфейс хотспота с собственным выводом `iw list` этой машины, а канал
с тем, что радио сообщило как пригодное.

### Где живёт состояние и кто его пишет

Два писателя, два файла, ни одного общего файла. Ни один процесс не пишет в файл
другого, поэтому нет ни блокировок, ни потерянных обновлений, от которых надо
было бы защищаться. [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md), раздел «Who writes what», фиксирует это
решение и более ранний черновик, который оно отменило.

```mermaid
flowchart TB
    subgraph panelowns["Пишет только caspian serve --panel"]
        SJ["/var/lib/caspian/state.json<br/>0600 caspian. Хранит вставленный конфиг<br/>и пароль хотспота"]
    end

    subgraph privowns["Пишет только caspian serve --privileged"]
        JN["/var/lib/caspian/netcfg.journal<br/>0600 root. Обратное действие для каждого изменения,<br/>записанное до самого изменения"]
        HC["/run/caspian/hostapd.conf<br/>0600 root, tmpfs, перезаписывается при каждом старте"]
        DC["/run/caspian/dnsmasq.conf<br/>0600 root, tmpfs, перезаписывается при каждом старте"]
    end

    subgraph nofile["Живёт в памяти и не пишется ни в один файл"]
        CUT["отсечка"]
        EVT["список событий панели"]
        RING["кольцевой буфер лога движка"]
    end
```

Привилегированная сторона не читает вообще ни одного файла состояния. Всё, что
ей нужно, приходит в запросе на старт. `TestPrivsvcReadsNoStateFile` сканирует
исходники самого этого пакета и падает, если он когда-нибудь начнёт читать такой
файл, чего комментарий обеспечить бы не смог.

Полная таблица путей, прав и владельцев находится в [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md). Порты
зафиксированы там же: 53 для клиентского DNS на хотспоте, 5354 на loopback для
DNS-слушателя движка, 8088 для панели, 10808 на loopback для диагностического
входящего SOCKS.

## Как движутся данные

### Вставленная share-ссылка становится работающим туннелем

`startNow` в [`internal/panel/handlers.go`](https://github.com/Iman/caspian/blob/main/internal/panel/handlers.go) документирует порядок, и именно
порядок позволяет различить три вида отказов конфига. Ничто на машине не
трогается, пока не пройдены и состояние 1, и состояние 2.

```mermaid
sequenceDiagram
    autonumber
    participant U as Человек за панелью
    participant PA as internal/panel
    participant LK as internal/link
    participant EN as internal/engine
    participant PS as internal/privsvc, root
    participant NC as internal/netcfg
    participant HS as internal/hotspot

    U->>PA: POST /power, on=1
    PA->>LK: link.Parse сохранённого текста
    Note over LK: Состояние 1. Не разобралось.<br/>Пользователь должен исправить текст.
    LK-->>PA: Link, не хранящий учётных данных ни в одном экспортированном поле
    PA->>LK: Link.XrayConfig
    LK-->>PA: один outbound с тегом proxy, без null-значений
    PA->>EN: engine.Validate
    Note over EN: Состояние 2. Прочитано и непригодно как написано.<br/>Ни один сокет не открывается. Никуда не звонят.
    PA->>PS: StartRequest через priv.sock
    PS->>PS: нижняя граница часов, повторный разбор, проверка против этой машины
    PS->>NC: Detect, затем PlanNetwork
    PS->>PS: xcfg.Build, затем снова engine.Validate
    PS->>NC: Применить PreEngineSteps. Файрвол идёт первым.
    PS->>NC: AssertHotspotInterfaceReleased
    PS->>EN: Engine.Start. Здесь появляется устройство туннеля.
    PS->>NC: Применить PostEngineSteps. Каждая команда называет туннель.
    PS->>HS: Supervisor.Start: сначала hostapd, затем dnsmasq
    PS->>NC: AssertHotspotIsAccessPoint
    PS->>PS: прозондировать сервер
    Note over PS: Состояние 3. Со ссылкой всё было в порядке,<br/>а сервер не ответил. Отката нет:<br/>коробка полностью настроена и блокирует.
    PS-->>PA: nil или один panel.Fault
```

Три детали в этой последовательности несут нагрузку.

Документ для движка собирается дважды и по разным причинам. `internal/link`
производит outbound и ничего больше. `internal/xcfg` производит всё вокруг него:
входящий TUN, на который приходит клиентский трафик, входящий SOCKS на loopback,
локальный DNS-слушатель, политику резолверов и правила маршрутизации. Ничто из
этого не берётся из того, что прислал вызывающий.

Старт, упавший на полпути, отменяется полностью. В журнале уже лежит обратное
действие для каждого изменения, записанное на диск до того, как изменение
достигло ядра. Упавший старт оставляет машину такой, какой её нашли.

Сервер, который не отвечает, это не полунастроенная коробка. Все изменения
удались, файрвол действует, а пересылаемый клиентский трафик заблокирован,
потому что туннель ничего не несёт. Поэтому об ошибке сообщается и ничего не
разбирается.

### Сетевой путь клиентского пакета

```mermaid
flowchart TB
    DEV["Подключённое устройство<br/>адрес выдан dnsmasq"] --> IF["Интерфейс хотспота"]
    IF --> PRE["цепочка nft prerouting, type nat<br/>DNS на порту 53 перенаправляется сюда"]
    PRE --> ROUTE{"Решение о маршрутизации<br/>ip rule from подсеть хотспота<br/>lookup table 8410"}
    ROUTE -->|"маршрут через туннель есть"| TOTUN["oif это устройство туннеля<br/>маршрут по умолчанию в таблице 8410"]
    ROUTE -->|"маршрут через туннель убран"| TOUP["oif это аплинк"]
    TOTUN --> FW1["цепочка nft forward, policy drop"]
    TOUP --> FW2["цепочка nft forward, policy drop"]
    FW1 -->|"iifname hotspot oifname tunnel<br/>ip saddr подсеть хотспота, accept"| POST["цепочка nft postrouting<br/>намеренно пустая, никакого masquerade"]
    FW2 -->|"iifname hotspot oifname uplink, drop<br/>блок против утечки, первое правило в цепочке"| DROP["отброшено"]
    POST --> TUN["Устройство туннеля<br/>сетевой стек в пространстве пользователя внутри движка"]
    TUN --> OB["outbound с тегом proxy"]
    OB --> UP["Аплинк<br/>закреплённый хостовый маршрут до сервера"]
    UP --> SRV["Ваш сервер"]
```

Блок против утечки называет только хотспот и аплинк. Он не может перестать
работать при исчезновении туннеля, потому что туннель в нём не упоминается.
Каждое правило, разрешающее клиентский трафик, туннель называет, поэтому такие
правила перестают срабатывать, а политика отбрасывает всё.

Каждый интерфейс сопоставляется по имени и никогда по индексу. Индекс
разрешается при загрузке набора правил, поэтому набор, называющий туннель по
индексу, не может загрузиться, пока туннель не поднят, а это ровно тот момент,
когда он должен действовать.

Цепочка postrouting пуста намеренно. Masquerade в сторону аплинка, это та самая
единственная строка, которая тихо превратила бы устройство в обычный роутер.

### Что происходит с этим путём, когда туннель исчезает

```mermaid
flowchart TB
    GONE["Туннель перестаёт нести трафик"] --> Q{"Устройство ещё существует?"}
    Q -->|"устройство удалено"| WD["Ядро убирает все маршруты через него"]
    WD --> FB["Клиентский трафик откатывается к основной таблице<br/>и уходит в сторону аплинка"]
    FB --> LB["Срабатывает блок против утечки: iifname hotspot oifname uplink, drop"]
    Q -->|"устройство осталось, но его никто не обслуживает"| ENTER["Трафик входит в устройство туннеля"]
    ENTER --> NOWHERE["Его никто не читает. Дальше он не идёт."]
    LB --> SAFE["Клиентский трафик наружу не выходит"]
    NOWHERE --> SAFE
```

Какая из ветвей происходит на деле, не установлено.
[`internal/netcfg/testdata/PROVENANCE.md`](https://github.com/Iman/caspian/blob/main/internal/netcfg/testdata/PROVENANCE.md) фиксирует наблюдение на целевой машине
от 2026-08-30: `xray0` присутствовал в списке устройств NetworkManager при
выключенной службе, со статусом `connected (externally)`. Почему так, здесь
никто не установил, а движок, это не код этого проекта. Ни одна из ветвей не
течёт, и ни одна не зависит от того, знаем ли мы, какая именно случится. Именно
поэтому блок написан так, чтобы называть только хотспот и аплинк.

### Путь DNS, который не совпадает с путём трафика

Вот здесь люди чаще всего ошибаются. DNS-запрос клиента не просто разрешается.
Он перехватывается.

```mermaid
flowchart TB
    ASK["Подключённое устройство спрашивает тот резолвер, который ему назвали,<br/>или зашитый в него самого, на порту 53"]
    ASK --> RD["nft prerouting на хотспоте:<br/>udp dport 53 и tcp dport 53 перенаправляются на :53<br/>Адрес назначения переписывается на эту коробку"]
    RD --> DM["dnsmasq, привязанный к интерфейсу хотспота<br/>/run/caspian/dnsmasq.conf"]
    DM -->|"единственный разрешённый ему upstream, это адрес loopback"| LD["DNS-слушатель движка<br/>127.0.0.1:5354, входящий тег local-dns-in"]
    LD --> R1["правило ruleTagLocalDNS<br/>inboundTag local-dns-in, outbound dns-out"]
    R1 --> APP["DNS-приложение движка<br/>резолверы из internal/xcfg/resolvers.go"]
    APP --> R2["правило ruleTagResolvers<br/>inboundTag resolver-in, outbound proxy.<br/>Выше правила о приватных адресах"]
    R2 --> OB["outbound с тегом proxy"]
    OB --> EXIT["цепочка резолверов, достигаемая с дальнего конца туннеля"]
```

Четыре свойства этой цепочки, и то, что каждое из них держит.

Перенаправление переписывает адрес назначения, поэтому устройство с зашитым в
него резолвером получает ответ здесь, а не выпускается наружу к тому резолверу,
который ему назвали. Сценарий: «a client cannot reach a resolver of its own
choosing».

Предложение DHCP называет эту коробку один раз и никакого другого резолвера. Это
заслуживает отдельного сценария, потому что ошибка здесь невидима:
перенаправление всё равно переписало бы пакеты, поэтому в сети ничего бы не
выглядело неправильным. Сценарий: «the box offers itself as the resolver and
never names another».

`internal/hotspot` отклоняет любой upstream для dnsmasq, который не является
адресом loopback. Цель вне loopback означала бы запрос, покидающий коробку в
обход туннеля, для каждого имени, которое спрашивает каждый клиент. Отвечает там
слушатель движка, и `TestLocalDNSDefaultMatchesTheHotspotUpstream` падает, если
эти два порта разойдутся. [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md) называет эту пару той, что ломается
тихо: если они разойдутся, все подключённые устройства перестанут резолвить, при
том что и хотспот, и туннель будут выглядеть здоровыми.

Правило, отправляющее собственные запросы резолвера в туннель, стоит выше
правила, отправляющего приватные адреса напрямую. Поэтому резолвер на приватном
адресе всё равно достигается через туннель, а не по локальной сети.
`TestLocalDNSQueriesCannotFallOutToTheUplink` и `TestPrivateRangesRouteDirect`
держат эти две половины.

Сама цепочка резолверов, это три оператора в трёх юрисдикциях: фильтрующий
сервис Quad9, вариант FAMILY у Cloudflare и CleanBrowsing Security.
[`internal/xcfg/resolvers.go`](https://github.com/Iman/caspian/blob/main/internal/xcfg/resolvers.go) фиксирует, почему выбран каждый из них и какой
почти идентичный адрес того же оператора выбран намеренно не был. Ни в одной
настройке по умолчанию не появляется резолвер Google, и
`TestNoGoogleAnywhereInGeneratedConfigs` сканирует на него каждый генерируемый
документ.

Остальные порты обработаны, и один из них обработать нельзя:

```mermaid
flowchart LR
    DOT["DNS over TLS<br/>tcp 853"] --> REJ["reject с tcp reset,<br/>чтобы устройство откатилось на порт 53"]
    DOQ["DNS over QUIC<br/>udp 853"] --> DRP["drop"]
    DOH["DNS over HTTPS<br/>порт 443"] --> CAR["несётся через туннель, как любой HTTPS.<br/>Не утечка. И ничему здесь не видно."]
```

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)
