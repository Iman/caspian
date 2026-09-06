# Лицензия и используемые проекты

[English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh)

[Вики Caspian](https://github.com/Iman/caspian/wiki/Home.ru)

> Руководство перенесено из README. Даты измерений сохранены; перенос документации не означает нового запуска тестов.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## Лицензия

AGPL-3.0-or-later, с тремя дополнительными условиями по разделу 7. Все три
относятся к тем, что раздел 7 допускает, и ни одно не ограничивает того, что вы
можете делать с программой: сохранять уведомление об авторском праве, эту
атрибуцию и видимое упоминание проекта Caspian в любом пользовательском
интерфейсе; помечать вашу версию как изменённую, если вы её меняете; и не
использовать имена авторов или проекта в целях рекламы, что включает сбор
пожертвований, спонсорских средств или грантов под этими именами. Полный текст
находится в [`LICENSE`](https://github.com/Iman/caspian/blob/main/LICENSE), а условия в [`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE).

Третье условие ограничивает использование ИМЁН и ничего больше. Вы по-прежнему
свободны запускать, изучать, изменять и распространять программу на условиях
AGPL, для любых целей, включая коммерческие. Чего вам нельзя, так это собирать
деньги от имени авторов.

AGPL, а не GPL, потому что эта программа обычно работает как сервис, к которому
подключаются другие люди, и раздел 13 закрывает пробел, который оставляет
обычная GPL. Не разрешительная лицензия, потому что бинарник статически
линкуется с кодом под GPL-3.0-or-later: `github.com/sagernet/sing` и
`github.com/sagernet/sing-shadowsocks`, оба достижимые через xray-core. Поэтому
совокупная работа должна быть на условиях семейства GPL, а MIT или Apache-2.0
для неё недоступны.

## На чём это построено

Caspian, это небольшое количество кода вокруг работы других людей. Движок, это
xray-core, а парсер share-ссылок принадлежит XTLS. Ни один из этих проектов не
поддерживает этот; они указаны потому, что работа их.

| Проект | Лицензия | Что он здесь делает |
|---|---|---|
| [xray-core](https://github.com/xtls/xray-core) | MPL-2.0 | Прокси-движок, слинкованный внутрь процесса, а не запускаемый отдельной программой |
| [libXray](https://github.com/XTLS/libXray) | MIT | Парсер share-ссылок, вендоренный в `third_party/libxray-share/` |
| [REALITY](https://github.com/xtls/reality) | MPL-2.0 | Транспорт с маскировкой под TLS |
| [uTLS](https://github.com/refraction-networking/utls) | BSD-3-Clause | Подражание отпечаткам TLS |
| [quic-go](https://github.com/apernet/quic-go) | MIT | Стек QUIC, на котором работает Hysteria2 |
| [gVisor](https://github.com/google/gvisor) | Apache-2.0 | Сетевой стек в пространстве пользователя, который использует входящий TUN |
| [sing](https://github.com/sagernet/sing) и [sing-shadowsocks](https://github.com/sagernet/sing-shadowsocks) | GPL-3.0-or-later | Shadowsocks 2022 и причина, по которой этот проект копилефтный |
| [netlink](https://github.com/vishvananda/netlink) | Apache-2.0 | Интерфейсы, адреса и маршруты |
| [miekg/dns](https://github.com/miekg/dns) | BSD-3-Clause | Обработка сообщений DNS |
| [gorilla/websocket](https://github.com/gorilla/websocket) | BSD-2-Clause | Транспорт WebSocket |
| [CIRCL](https://github.com/cloudflare/circl) | BSD-3-Clause | Постквантовый обмен ключами |

Ему также нужны `hostapd`, `dnsmasq`, `nftables`, `iw` и `iproute2` на машине.
Они работают как отдельные программы, а не линкуются внутрь, поэтому их лицензии
на эту не влияют, но без них устройство ничего собой не представляет.

[`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE) несёт полную запись: каждый модуль в бинарнике, лицензию, прочитанную
из его собственного файла лицензии, и рассуждение о совместимости.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

## Компоненты Windows

| Проект | Лицензия | Назначение |
|---|---|---|
| [Wintun](https://www.wintun.net/) | Wintun Prebuilt Binaries License | Подписанный драйвер туннеля `wintun.dll` |
| [.NET runtime and Windows Forms](https://github.com/dotnet/runtime) | MIT | Среда выполнения приложения управления Windows |
| `System.ServiceProcess.ServiceController` | MIT | Управление службами из `CaspianControl.exe` |

Установка Windows содержит отдельный файл `wintun.dll`. Официальный подписанный Wintun 0.14.1 распространяется без изменений.
Лицензия находится в [`third_party/wintun/PREBUILT-BINARIES-LICENSE.txt`](https://github.com/Iman/caspian/blob/main/third_party/wintun/PREBUILT-BINARIES-LICENSE.txt) и при установке копируется в `C:\Program Files\Caspian\WINTUN-LICENSE.txt`.

Файлы `caspian-tethering.exe` и `CaspianControl.exe` содержат среду выполнения .NET внутри исполняемых файлов.
Лицензии и уведомления находятся в `third_party/dotnet/`. Пакет ссылок Windows SDK нужен для сборки и не устанавливается вместе с Caspian.
