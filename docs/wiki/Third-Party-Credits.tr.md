<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Third-Party-Credits) · [فارسی](https://github.com/Iman/caspian/wiki/Third-Party-Credits.fa) · [Русский](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ru) · [简体中文](https://github.com/Iman/caspian/wiki/Third-Party-Credits.zh) · [العربية](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ar) · [**Türkçe**](https://github.com/Iman/caspian/wiki/Third-Party-Credits.tr) · [اردو](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ur)

</div>

<div dir="ltr" lang="tr">

[Caspian vikisi](https://github.com/Iman/caspian/wiki/Home.tr) · [Sorun giderme](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

</div>
<!-- wiki-navigation:end -->

<a id="third-party-code-and-credits"></a>
# Üçüncü taraf kodu ve kredileri

[NOTICE](https://github.com/Iman/caspian/blob/feature/sni/NOTICE), Caspian'ın bağlantılı kitaplıklarını ve dağıtım dosyalarını listeler.
Her yukarı akış bileşeni kendi lisansını korur.
Caspian'ın AGPL şartları bu yukarı yönlü bildirimlerin yerine geçmez.

<a id="sni-spoofing"></a>
## SNI sahteciliği

Ana kod ve fikir **[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)**'den alınmıştır.
Caspian, `13b78cf7e073f38d9cadcff542faf4a00b0a6de2` taahhüdünü inceledi.
ClientHello şablonu ve el sıkışma algoritması `internal/snispoof`'yi bilgilendirir.
Caspian'daki değişiklikler arasında Go entegrasyonu, doğrulama, bağlantı sahipliği, kaynak sınırları, geri alma ve testler yer alıyor.

Türetilmiş kaynak dosyaları GPL-3.0-only bildirimlerini korur.
[yukarı akış GPL lisansı](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/LICENSE.txt) ve [kaynak ve yazar bilgileri](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/README.md)'nin tamamı depoda kalır.
GPLv3 bölüm 13, her parça kendi koşullarını korurken AGPLv3 koduyla kombinasyona izin verir.
Distribütörler, geçerli lisanslar kapsamında bildirimleri saklamalı, değişiklikleri işaretlemeli ve ilgili kaynağı sağlamalıdır.
Kredi, yukarı yöndeki yazarların onayladığı anlamına gelmez.

Windows x64, Basil (basil00) ve katkıda bulunanların **[WinDivert](https://github.com/basil00/WinDivert/tree/v2.2.2)** ürününü kullanır.
Caspian ikili lisansından LGPL-3.0'yi seçiyor.
Yükleyici, v2.2.2 için değiştirilmemiş sürücüyü ve DLL'yi, tam lisans paketini, ilişkilendirmeyi ve kaynak arşivini içerir.
Bkz. [WinDivert dağıtım ayrıntıları](https://github.com/Iman/caspian/blob/feature/sni/third_party/windivert/README.md).
Windows ARM64, WinDivert'i içermez ve bu SNI özelliğini kullanamaz.

<a id="ideas-and-acknowledgements"></a>
## Fikirler ve teşekkür

Caspian ayrıca SNI çalışmalarına bilgi sağlayan fikirler ve uygulama karşılaştırmaları için bu projelerin yazarlarına ve katkıda bulunanlara teşekkür ediyor.
Kodları ve yürütülebilir dosyaları paketlenmemiştir.

| Proje | İncelenen revizyon | Lisans ve kullanım |
| --- | --- | --- |
| [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof) | `a0d68c6627e0898cf13176e322420743f69d894f` | Lisans dosyası bulunamadı; yalnızca karşılaştırma |
| [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) | `f593a276f9ec753889f80208c6a7c5cf455df94a` | Apache-2.0; strateji referansı |
| [bol-van/zapret](https://github.com/bol-van/zapret) | `87e058624c72863db53bdaf7fb6f16576dddb6ab` | MIT; strateji ve teşhis referansı |
| [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android) | `c68e350f5f9e9cff308c289db261d7c5d034efa2` | Üst düzey uygulama lisansı bulunamadı; yalnızca fikirler |
| [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust) | `d2956025c31d96f0f0a341af4f1a8eda204857c7` | MIT'yi beyan eder; GPL şablonunun kaynağının açıklığa kavuşturulması gerekiyor; kod kopyalanmadı |

<a id="other-distributed-components"></a>
## Diğer dağıtılmış bileşenler

[paylaşım bağlantısı ayrıştırıcısı](https://github.com/Iman/caspian/blob/feature/sni/third_party/libxray-share/LICENSE), MIT lisansını korur.
Windows yükleyicileri ayrıca resmi Wintun ikili dosyalarını ve bağımsız .NET yardımcılarını da içerir.
Bildirimleri [third_party](https://github.com/Iman/caspian/blob/feature/sni/third_party) altında kalır ve uygulamanın yanına yüklenir.
Go Wintun bağlaması, Windows'ta bir MIT çalışma zamanı bağımlılığıdır.

<!-- English-source-sha256: ceef83c2b7b0779eb04c1fa1c854f35aaf685978f4d17f439ddaa2fd2d01847c -->
