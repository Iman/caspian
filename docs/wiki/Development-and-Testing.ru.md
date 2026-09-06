# Разработка и тестирование

[English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) | [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) | [中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh)

[Вики Caspian](https://github.com/Iman/caspian/wiki/Home.ru)

> Руководство перенесено из README. Даты измерений сохранены; перенос документации не означает нового запуска тестов.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## Как это запускать

Соберите бинарник и передайте его установщику. Этому пути не нужен никакой
релиз, и установщик принимает его как для настоящей установки, так и для
пробного прогона:

    go build -o /tmp/caspian-linux-arm64 ./cmd/caspian
    sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

    env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
        CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
        bash install.sh --dry-run --yes

Уберите `--dry-run`, чтобы установить по-настоящему. Без
`CASPIAN_LOCAL_CHECKSUMS` установщик предупреждает, ровно этими словами, что он
устанавливает непроверенный бинарник. [`docs/INSTALL.md`](https://github.com/Iman/caspian/blob/main/docs/INSTALL.md), это полное руководство.
В нём есть и подставной `uname` для того, чтобы пройти все отказы на машине, на
которую установка невозможна.

У бинарника четыре подкоманды:

    caspian serve --privileged     root: routes, firewall, access point, engine
    caspian serve --panel          the caspian user: the web panel, nothing privileged
    caspian check                  report what this box looks like; changes nothing
    caspian version

Подкоманды, которая применяла бы конфиг или дёргала переключатель, намеренно
нет. CLI говорит об этом сам: "After the installer has run, everything a person
does happens in the panel."

[`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) удаляет юниты, бинарник и каталоги и проигрывает сетевой журнал,
так что коробка остаётся такой, какой её нашли. Прочитайте дефект D5 ниже,
прежде чем на это полагаться.

## Правила, которым проект следует сам

Это не пожелания. У каждого правила есть механизм, и механизм назван.

**Ничто не называется работающим без адреса выхода, зафиксированного из
настоящего трафика.** [`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md), раздел 6. Соединение, это не
результат. Аппаратный стенд ставит оценку UNPROVEN, а не PASS, когда адрес
выхода не зафиксирован, и завершается с кодом 1.

**Уверенно написанная неверная фраза хуже, чем отсутствие фразы.** Читатель,
которому сказали, что нечто обрабатывается правильно, делает вывод, что
проверять нечего. Поэтому исправление оставляет после себя тест, а не более
удачную формулировку. `TestNothingInTheApplianceWatchesTheUplink` существует
потому, что два документа когда-то утверждали, будто коробка следит за своим
аплинком и перезагружает файрвол, когда тот меняется.

**Запущенный процесс не является доказательством того, что он сработал.**
Интерфейс хотспота считывается обратно из ядра, прежде чем к нему что-либо
привяжется, и точка доступа считывается обратно, прежде чем служба сообщит о
себе как о работающей. Оба обратных считывания добавлены после одного
измеренного случая, в котором каждая команда вернула успех.

**Каждый сценарий видели падающим.** `TestEveryScenarioCanFail` внедряет
поименованный дефект в каждое поведение и требует, чтобы оно стало красным.
Тест, который никто не видел падающим, это зелёная лампочка, подключённая в
пустоту.

**Происхождение фикстуры записано в её имени файла.** `capture-pi5-`, это
побайтовый вывод настоящей команды на целевой машине, `scenario-`, это машина,
которую никто не измерял, а `golden-`, это собственный вывод этого проекта.
Тест, читающий файл `capture-pi5-`, делает утверждение о целевой машине. Тест,
читающий файл `scenario-`, не делает.

**Учётные данные в коммите остаются там навсегда.** `test/goldenscan` прочёсывает
каждую закоммиченную фикстуру на предмет зарегистрированных маркеров и форм
учётных данных, и проверяет имена файлов так же, как их содержимое. Его видели
ловящим подложенный секрет каждого известного ему класса.

**Пороги покрытия работают как храповик.** Каждое число в [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh), это
то, что пакет намерил после работы, которая его туда привела, а не цель, на
которую кто-то надеялся. Пакет без строки не контролируется порогом, и
отсутствие строки означает «порог ещё не согласован», а не «этот пакет покрыт».

**Привилегированная сторона не доверяет ничему из того, что присылает
вызывающий.** Каждое поле каждого запроса сверяется с тем, что эта машина
определила сама. Отказ, это код ошибки из закрытого множества, никогда не фраза
и никогда не значение, присланное вызывающим.

**Коробка ничего не просит у интернета.** Ни телеметрии, ни звонков домой, ни
отправки отчётов о падениях, ни веб-шрифтов, ни файлов гео-данных, ни резолвера
Google ни в одной настройке по умолчанию.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[Architecture](https://github.com/Iman/caspian/wiki/Architecture) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting)
