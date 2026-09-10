#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Shared wiki navigation and localized labels. Run with --check or --write."""

import argparse
import hashlib
from pathlib import Path
import re

BASE = "https://github.com/Iman/caspian/wiki/"
LANGUAGES = ("en", "fa", "ru", "zh", "ar", "tr", "ur")
NAMES = ("English", "فارسی", "Русский", "简体中文", "العربية", "Türkçe", "اردو")
RTL = {"fa", "ar", "ur"}
TOPICS = (
    "Getting-Started", "Install-Linux", "Install-macOS", "Install-Windows",
    "Installation", "Panel-and-Configuration", "Troubleshooting",
    "Protocols-and-Transports", "SNI-Spoofing", "Security-and-Privacy",
    "Releases-and-Maintenance", "Architecture", "Development-and-Testing",
    "Documentation-Map", "Translations", "Page-Template", "Licence-and-Credits",
    "Third-Party-Credits",
)
LABELS = {
    "en": "Getting started|Linux and Raspberry Pi|macOS|Windows|Installation reference|Panel and configuration|Troubleshooting|Protocols and transports|SNI spoofing and TLS splitting|Security and privacy|Updates and maintenance|Architecture and data flow|Development and testing|Documentation map|Translations|Page template|License and credits|Third-party code and credits",
    "fa": "شروع کار|لینوکس و رزبری پای|macOS|ویندوز|مرجع نصب|پنل و پیکربندی|عیب‌یابی|پروتکل‌ها و روش‌های انتقال|جعل SNI و تقسیم TLS|امنیت و حریم خصوصی|به‌روزرسانی و نگهداری|معماری و جریان داده|توسعه و آزمایش|نقشه مستندات|ترجمه‌ها|الگوی صفحه|مجوز و قدردانی|کد شخص ثالث و قدردانی",
    "ru": "Начало работы|Linux и Raspberry Pi|macOS|Windows|Справочник по установке|Панель и настройка|Устранение неполадок|Протоколы и транспорт|Подмена SNI и разделение TLS|Безопасность и конфиденциальность|Обновления и обслуживание|Архитектура и поток данных|Разработка и тестирование|Карта документации|Переводы|Шаблон страницы|Лицензия и благодарности|Сторонний код и благодарности",
    "zh": "开始使用|Linux 和 Raspberry Pi|macOS|Windows|安装参考|面板与配置|故障排除|协议与传输方式|SNI 伪装与 TLS 分片|安全与隐私|更新与维护|架构与数据流|开发与测试|文档索引|翻译|页面模板|许可证与致谢|第三方代码与致谢",
    "ar": "البدء|Linux و Raspberry Pi|macOS|Windows|مرجع التثبيت|اللوحة والإعدادات|استكشاف الأخطاء وإصلاحها|البروتوكولات وطرق النقل|تمويه SNI وتقسيم TLS|الأمان والخصوصية|التحديثات والصيانة|البنية وتدفق البيانات|التطوير والاختبار|خريطة التوثيق|الترجمات|قالب الصفحة|الترخيص والشكر|أكواد الجهات الخارجية والشكر",
    "tr": "Başlangıç|Linux ve Raspberry Pi|macOS|Windows|Kurulum başvurusu|Panel ve yapılandırma|Sorun giderme|Protokoller ve taşıma yöntemleri|SNI yanıltma ve TLS bölme|Güvenlik ve gizlilik|Güncellemeler ve bakım|Mimari ve veri akışı|Geliştirme ve test|Belge haritası|Çeviriler|Sayfa şablonu|Lisans ve teşekkürler|Üçüncü taraf kodu ve teşekkürler",
    "ur": "آغاز|Linux اور Raspberry Pi|macOS|Windows|تنصیب کا حوالہ|پینل اور ترتیبات|مسائل کا حل|پروٹوکول اور نقل و حمل کے طریقے|SNI کی جعل سازی اور TLS کی تقسیم|سلامتی اور رازداری|اپ ڈیٹس اور دیکھ بھال|ساخت اور ڈیٹا کا بہاؤ|تیاری اور جانچ|دستاویزات کا نقشہ|تراجم|صفحے کا سانچہ|لائسنس اور اعترافات|تیسرے فریق کا کوڈ اور اعترافات",
}
UI = {
    "en": ("Caspian wiki", "Set up Caspian", "Use and maintain", "Develop and contribute", "Choose your operating system to install Caspian, or open Troubleshooting for connection problems."),
    "fa": ("ویکی کاسپین", "راه‌اندازی کاسپین", "استفاده و نگهداری", "توسعه و مشارکت", "برای نصب کاسپین، سیستم‌عامل خود را انتخاب کنید. برای رفع مشکلات اتصال، راهنمای عیب‌یابی را باز کنید."),
    "ru": ("Вики Caspian", "Установка Caspian", "Использование и обслуживание", "Разработка и участие", "Выберите свою операционную систему для установки Caspian. При проблемах с подключением откройте раздел устранения неполадок."),
    "zh": ("Caspian 文档", "安装 Caspian", "使用与维护", "开发与贡献", "选择操作系统以安装 Caspian。如遇连接问题，请查看故障排除指南。"),
    "ar": ("ويكي Caspian", "إعداد Caspian", "الاستخدام والصيانة", "التطوير والمساهمة", "اختر نظام التشغيل لتثبيت Caspian، أو افتح دليل استكشاف الأخطاء وإصلاحها لحل مشكلات الاتصال."),
    "tr": ("Caspian vikisi", "Caspian kurulumu", "Kullanım ve bakım", "Geliştirme ve katkı", "Caspian kurulumu için işletim sisteminizi seçin. Bağlantı sorunları için sorun giderme kılavuzunu açın."),
    "ur": ("Caspian ویکی", "Caspian کی تنصیب", "استعمال اور دیکھ بھال", "تیاری اور تعاون", "Caspian انسٹال کرنے کے لیے اپنا آپریٹنگ سسٹم منتخب کریں۔ کنکشن کے مسائل کے لیے مسائل کے حل کی رہنما کھولیں۔"),
}
GROUPS = ((0, 5), (5, 11), (11, 18))
START = "<!-- wiki-navigation:start -->"
END = "<!-- wiki-navigation:end -->"


def slug(topic, language):
    return topic + (f".{language}" if language != "en" else "")


def link(topic, language, label):
    return f"[{label}]({BASE}{slug(topic, language)})"


def navigation(topic, language):
    choices = " · ".join(
        link(topic, lang, f"**{name}**" if lang == language else name)
        for lang, name in zip(LANGUAGES, NAMES)
    )
    local = " · ".join((link("Home", language, UI[language][0]),
                        link("Troubleshooting", language, LABELS[language].split("|")[6])))
    direction = "rtl" if language in RTL else "ltr"
    return f'{START}\n<div dir="ltr">\n\n{choices}\n\n</div>\n\n<div dir="{direction}" lang="{language}">\n\n{local}\n\n</div>\n{END}\n\n'


def directory(language):
    labels = LABELS[language].split("|")
    sections = []
    for heading, (start, stop) in zip(UI[language][1:4], GROUPS):
        sections.append(f"## {heading}\n\n" + "\n".join(
            "- " + link(TOPICS[i], language, labels[i]) for i in range(start, stop)))
    return "\n\n".join(sections)


def home(language, topic="Home"):
    direction = "rtl" if language in RTL else "ltr"
    return (navigation(topic, language) + f'<div dir="{direction}" lang="{language}">\n\n'
            + f"# {UI[language][0]}\n\n{UI[language][4]}\n\n"
            + directory(language) + "\n\n</div>\n")


def sidebar():
    # GitHub has one global sidebar; locale suffixes do not select a sidebar.
    parts = ["<!-- Generated by scripts/wiki_navigation.py. -->", "### Caspian"]
    for lang, name in zip(LANGUAGES, NAMES):
        direction = "rtl" if lang in RTL else "ltr"
        labels = LABELS[lang].split("|")
        items = ["- " + link("Home", lang, UI[lang][0])]
        items.extend("- " + link(TOPICS[i], lang, labels[i]) for i in (0, 1, 2, 3, 5, 6))
        parts.append(f'<details>\n<summary>{name}</summary>\n\n<div dir="{direction}" lang="{lang}">\n\n'
                     + "\n".join(items) + "\n\n</div>\n\n</details>")
    return "\n\n".join(parts) + "\n"


def render(root):
    result = {"_Sidebar.md": sidebar()}
    for language in LANGUAGES:
        for topic in ("Home", "Caspian-wiki"):
            result[slug(topic, language) + ".md"] = home(language, topic)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--write", action="store_true")
    parser.add_argument("directory", nargs="?", type=Path, default=Path(__file__).resolve().parents[1] / "docs/wiki")
    args = parser.parse_args()
    errors = []
    for name, expected in render(args.directory).items():
        path = args.directory / name
        if name != "_Sidebar.md" and name.rsplit(".", 2)[-2] in LANGUAGES[1:]:
            source = name.rsplit(".", 2)[0] + ".md"
            digest = hashlib.sha256(render(args.directory)[source].encode()).hexdigest()
            expected += f"\n<!-- English-source-sha256: {digest} -->\n"
        if args.write:
            path.write_text(expected, encoding="utf-8", newline="\n")
        elif not path.exists() or path.read_text(encoding="utf-8") != expected:
            errors.append(f"{name}: regenerate with python scripts/wiki_navigation.py --write")
    for error in errors:
        print(error)
    return bool(errors)


if __name__ == "__main__":
    raise SystemExit(main())
