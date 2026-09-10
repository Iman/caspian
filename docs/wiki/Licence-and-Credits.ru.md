<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh) | [العربية](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Licence-and-Credits.tr) | [اردو](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ur)

</div>

<a id="licence-and-credits"></a>
# Лицензия и кредиты



[Caspian вики](https://github.com/Iman/caspian/wiki/Home.ru)

> Это руководство взято из существующего README. Его измерения сохраняют свои первоначальные даты; этот шаг документации не сообщает о новом тестовом запуске.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="licence"></a>
## Лицензия

AGPL-3.0-or-later, с тремя дополнительными условиями в разделе 7. Все три имеют
раздел 7 разрешает и не ограничивает то, что вы можете делать с программным обеспечением:
сохранить уведомление об авторских правах, указание авторства и видимую ссылку на
Caspianский проект в любом пользовательском интерфейсе; отметьте свою версию как измененную, если вы
изменить его; и не используйте имена авторов или имена проектов в рекламных целях,
что включает в себя сбор пожертвований, спонсорства или грантов на эти имена.
полный текст находится в [`LICENSE`](https://github.com/Iman/caspian/blob/main/LICENSE), а условия — в [`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE).

Этот третий термин ограничивает использование ИМЕН и ничего больше. Вы остаетесь свободными
запускать, изучать, изменять и распространять программное обеспечение в соответствии с AGPL для любого
цели, в том числе коммерческой. Чего вы не можете делать, так это собирать деньги в
имя авторов.

AGPL, а не GPL, поскольку эта программа обычно работает как
сервис, к которому подключаются другие люди, и раздел 13 закрывает пробел в простой GPL
листья. Не разрешающая лицензия, поскольку двоичный файл статически связывается
Код GPL-3.0-or-later: `github.com/sagernet/sing` и
`github.com/sagernet/sing-shadowsocks`, оба получены через xray-core. Итак,
совместная работа должна осуществляться на условиях семейства GPL, а MIT или Apache-2.0 не являются
доступен для этого.

<a id="built-on"></a>
## Построен на

Каспиан — это небольшое количество кода вокруг работы других людей. Двигатель
xray-core, а анализатор общих ссылок — XTLS. Ни один проект этого не поддерживает
один; им доверяют, потому что это их работа.

| Проект | Лицензия | Что он здесь делает |
|---|---|---|
| [xray-core](https://github.com/xtls/xray-core) | MPL-2.0 | Прокси-движок, связанный внутри процесса, а не запускаемый как отдельная программа. |
| [libXray](https://github.com/XTLS/libXray) | MIT | Анализатор общих ссылок, поставляемый под номером `third_party/libxray-share/`. |
| [REALITY](https://github.com/xtls/reality) | MPL-2.0 | Камуфляжный транспорт TLS |
| [uTLS](https://github.com/refraction-networking/utls) | BSD-3-Clause | Имитация отпечатков пальцев TLS |
| [quic-go](https://github.com/apernet/quic-go) | MIT | Стек QUIC Hysteria2 работает на |
| [gVisor](https://github.com/google/gvisor) | Apache-2.0 | Сетевой стек пользовательского пространства, который использует входящий трафик TUN. |
| [sing](https://github.com/sagernet/sing) и [sing-shadowsocks](https://github.com/sagernet/sing-shadowsocks) | GPL-3.0-or-later | Shadowsocks 2022 г. и причина, по которой этот проект имеет авторское лево |
| [netlink](https://github.com/vishvananda/netlink) | Apache-2.0 | Интерфейсы, адреса и маршруты |
| [miekg/dns](https://github.com/miekg/dns) | BSD-3-Clause | обработка DNS-сообщений |
| [gorilla/websocket](https://github.com/gorilla/websocket) | BSD-2-Clause | Транспорт WebSocket |
| [CIRCL](https://github.com/cloudflare/circl) | BSD-3-Clause | Постквантовый обмен ключами |
| [Wintun](https://www.wintun.net/) | Лицензия на готовые двоичные файлы Wintun | Подписанный драйвер туннеля `wintun.dll` в Windows. |
| [Среда выполнения .NET и Windows Forms](https://github.com/dotnet/runtime) | MIT | Автономный помощник Windows и среда выполнения приложений в трее |
| `System.ServiceProcess.ServiceController` | MIT | Управление службами Windows из `CaspianControl.exe` |

Установка Windows включает `wintun.dll`. Сборка SNI также включает WinDivert для Windows x64.
Каспиан распространяет официальный подписанный бинарный файл Wintun 0.14.1 без изменений.
Его лицензия находится в
[`third_party/wintun/PREBUILT-BINARIES-LICENSE.txt`](https://github.com/Iman/caspian/blob/main/third_party/wintun/PREBUILT-BINARIES-LICENSE.txt) и копируется в
`C:\Program Files\Caspian\WINTUN-LICENSE.txt` во время установки.

`caspian-tethering.exe` и `CaspianControl.exe` являются автономными .NET.
программы. Их .NET-компоненты находятся внутри исполняемых файлов, а не рядом с ними.
их как дополнительные DLL. Лицензия .NET и уведомления находятся в `third_party/dotnet/`.
Справочный пакет Windows SDK является входными данными для сборки и не устанавливается вместе с
Caspian.

Также необходимы `hostapd`, `dnsmasq`, `nftables`, `iw` и `iproute2` на
машина. Они запускаются как отдельные программы, а не связаны друг с другом, поэтому их
лицензии на это не влияют, но без них устройство ничто.

[`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE) содержит полную запись: каждый модуль в двоичном виде, лицензия прочитана.
из собственного файла лицензии и соображений совместимости.



<!-- SNI upstream credits -->

Кредиты на подмену SNI: [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) (GPL-3.0) с WinDivert (LGPL-3.0) в Windows x64.
[Сторонние лицензии, исходные версии и авторство](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md).

<a id="sni-idea-acknowledgements"></a>
## Благодарность за идею SNI

Компания «Caspian» также благодарит авторов и участников этих проектов за идеи и сравнения реализаций, которые послужили основой для работы над SNI.
Их код и исполняемые файлы не включены в комплект.

- [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof): Сравнение пересылки Go SNI.
- [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust): реализация Rust SNI и сравнение функций.
- [bol-van/zapret](https://github.com/bol-van/zapret): стратегии и диагностика обхода DPI.
- [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI): Стратегии обхода DPI.
- [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android): Идеи интеграции Android SNI.

Адаптированный код сохраняет свою исходную лицензию и уведомления.
Благодарность за идею не дает разрешения на копирование кода и не подразумевает одобрения.
См. [сторонние кредиты](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ru) для ознакомления с проверенными версиями, лицензиями и областью использования.

<!-- Caspian guide navigation -->

Путеводители по Каспию: [настройка и поддерживаемые протоколы](https://github.com/Iman/caspian/wiki/Home.ru) · [Подмена SNI для обхода DPI: настройка и ограничения](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ru).

[WinDivert — Василий (basil00)](https://github.com/basil00/WinDivert/tree/v2.2.2): Windows x64, LGPL-3.0.


<!-- English-source-sha256: 55e2110d5bfd7485670033a177167abbb056f030356d7a78eb46204807b5452e -->
