<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Third-Party-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Third-Party-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Third-Party-Credits.zh) | [العربية](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Third-Party-Credits.tr) | [اردو](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ur)

</div>

<a id="third-party-code-and-credits"></a>
# Сторонний код и кредиты

[NOTICE](https://github.com/Iman/caspian/blob/feature/sni/NOTICE) перечисляет связанные библиотеки и файлы дистрибутива Каспиана.
Каждый вышестоящий компонент имеет собственную лицензию.
Условия AGPL проекта Caspian не заменяют лицензионные уведомления исходных проектов.

<a id="sni-spoofing"></a>
## SNI-спуфинг

Основной код и идея взяты из **[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)**.
Каспиан рассмотрел коммит `13b78cf7e073f38d9cadcff542faf4a00b0a6de2`.
Шаблон ClientHello и алгоритм установления связи информируют `internal/snispoof`.
Изменения Каспиана включают интеграцию Go, проверку, владение соединением, ограничения ресурсов, откат и тесты.

В производных исходных файлах сохраняются уведомления GPL-3.0-only.
Полные версии [исходная лицензия GPL](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/LICENSE.txt) и [указание авторства](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/README.md) остаются в репозитории.
Раздел 13 GPLv3 разрешает сочетание с кодом AGPLv3, при этом каждая часть сохраняет свои собственные условия.
Дистрибьюторы должны сохранять уведомления, отмечать изменения и предоставлять соответствующий источник в соответствии с применимыми лицензиями.
Благодарность не подразумевает одобрения со стороны вышестоящих авторов.

Windows x64 использует **[WinDivert](https://github.com/basil00/WinDivert/tree/v2.2.2)** от Бэзила (basil00) и его участников.
Компания «Caspian» выбирает LGPL-3.0 из своей двойной лицензии.
Установщик содержит немодифицированный драйвер и DLL, полный пакет лицензий, атрибуцию и исходный архив для версии 2.2.2.
См. [Подробности распространения WinDivert](https://github.com/Iman/caspian/blob/feature/sni/third_party/windivert/README.md).
Windows ARM64 не включает WinDivert и не может использовать эту функцию SNI.

<a id="ideas-and-acknowledgements"></a>
## Идеи и благодарности

Компания «Caspian» также выражает благодарность авторам и участникам этих проектов за идеи и сравнения реализаций, которые послужили основой для работы над SNI.
Их код и исполняемые файлы не включены в комплект.

| Проект | Рассмотренная версия | Лицензия и использование |
| --- | --- | --- |
| [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof) | `a0d68c6627e0898cf13176e322420743f69d894f` | Файл лицензии не найден; только сравнение |
| [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) | `f593a276f9ec753889f80208c6a7c5cf455df94a` | Apache-2.0; справочник по стратегии |
| [bol-van/zapret](https://github.com/bol-van/zapret) | `87e058624c72863db53bdaf7fb6f16576dddb6ab` | MIT; Справочник по стратегии и диагностике |
| [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android) | `c68e350f5f9e9cff308c289db261d7c5d034efa2` | Лицензия на приложение верхнего уровня не найдена; только идеи |
| [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust) | `d2956025c31d96f0f0a341af4f1a8eda204857c7` | Объявляет MIT; Происхождение шаблонов GPL требует разъяснения; код не скопирован |


<a id="other-distributed-components"></a>
## Другие распределенные компоненты

[парсер общих ссылок](https://github.com/Iman/caspian/blob/feature/sni/third_party/libxray-share/LICENSE) сохраняет лицензию MIT.
Установщики Windows также содержат официальные двоичные файлы Wintun и автономные помощники .NET.
Их уведомления остаются под [third_party](https://github.com/Iman/caspian/blob/feature/sni/third_party) и устанавливаются рядом с приложением.
Привязка Go Wintun — это зависимость времени выполнения MIT от Windows.

<!-- Caspian guide navigation -->

Caspianские гиды: [настройка и поддерживаемые протоколы](https://github.com/Iman/caspian/wiki/Home.ru) · [Подмена SNI для обхода DPI: настройка и ограничения](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ru).


<!-- English-source-sha256: 8a269a35df1d8ba95feb6a256569515780e472a71eb004eea9f0540dc32cc216 -->
