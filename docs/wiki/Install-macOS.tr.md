# macOS kurulumu

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/wiki/Install-macOS) | [فارسی](https://github.com/Iman/caspian/wiki/Install-macOS.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-macOS.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-macOS.zh) | [العربية](https://github.com/Iman/caspian/wiki/Install-macOS.ar) | [اردو](https://github.com/Iman/caspian/wiki/Install-macOS.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Install-macOS.tr)

</div>

[Bağlantı şemaları, önce kabloyu bağlama adımları, hizmetleri yeniden başlatma ve yaygın hatalar için ev kullanıcısı sorun giderme kılavuzunu okuyun.](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

İşlemci ve RAM: Caspian için gereken en düşük RAM miktarı, işlemci çekirdek sayısı ve saat hızı henüz ölçümlerle belirlenmedi. Kaynak kullanımı trafik miktarına, vekil sunucu protokolüne ve eşzamanlı bağlantı sayısına bağlıdır. Asgari gereksinimleri yayımlamadan önce boşta ve yük altında ölçüm yapılması gerekir.

macOS 13 veya sonrası ve yönetici hesabı gerekir. Yerleşik Wi-Fi erişim noktası olduğunda interneti Ethernet üzerinden sağlayın.

1. Resmî sürüm sayfasından işlemcinize uygun DMG dosyasını seçin: Apple Silicon için `arm64`, Intel için `amd64`.
2. Dosyayı açın, `Caspian.app` uygulamasını Applications klasörüne sürükleyin ve oradaki kopyayı açın.
3. Apple uygulamayı doğrulayamadığını bildirirse Done düğmesine basın. Dosyanın kaynağını kontrol ettikten sonra System Settings içindeki Privacy & Security bölümünden Caspian'ın yanındaki Open Anyway düğmesini seçin.
4. Yönetici parolasıyla kuruluma izin verin. İlk kurulumun gösterdiği panel parolasını saklayın.
5. Caspian is ready durumunu bekleyin. Open panel düğmesiyle paneli açıp Wi-Fi ve proxy ayarlarını girin.

Uygulamaya izin verdiğiniz hâlde arka plandaki `caspian` dosyası engelleniyorsa karantina özniteliği kalmış olabilir. Aşağıdaki komutu yalnızca doğrulanmamış geliştirici veya Apple noter onayı eksikliği uyarısında, kaynağı ve sağlama toplamını kontrol ettikten sonra kullanın.

Uyarı bir Trojan adı veriyorsa veya zararlı yazılım bildiriyorsa kurulumu durdurun ve bu komutu kullanmayın. Uyarının tam metnini, tespit adını, sürüm numarasını ve indirme bağlantısını bildirin.

```bash
sudo xattr -d com.apple.quarantine /usr/local/bin/caspian
```

Ardından Advanced options içinden Restart services seçeneğini kullanın. Komut yalnızca belirtilen dosyanın karantina özniteliğini kaldırır; dosyayı taramaz veya imzalamaz. `No such xattr`, özniteliğin bulunmadığını belirtir. Sorun sürerse diğer güvenlik kontrollerini kaldırmak yerine hatayı bildirin.

Mac, panel ve Wi-Fi parolaları farklıdır. Panel parolasını unutursanız Caspian Control içindeki Reset password seçeneğini yönetici izniyle kullanın.

[İngilizce ayrıntılar](https://github.com/Iman/caspian/wiki/Installation#macos-13-or-later)

[Caspian vikisi](https://github.com/Iman/caspian/wiki/Home.tr)

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

</div>

<!-- Caspian guide navigation -->

Caspian rehberleri: [kurulum ve protokoller](https://github.com/Iman/caspian/wiki/Home.tr) · [SNI spoofing ve DPI aşma: kurulum ve sınırlar (English)](https://github.com/Iman/caspian/wiki/SNI-Spoofing).
