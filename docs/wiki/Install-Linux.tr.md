# Linux ve Raspberry Pi kurulumu

[English](https://github.com/Iman/caspian/wiki/Install-Linux) | [فارسی](https://github.com/Iman/caspian/wiki/Install-Linux.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-Linux.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-Linux.zh) | [العربية](https://github.com/Iman/caspian/wiki/Install-Linux.ar) | [اردو](https://github.com/Iman/caspian/wiki/Install-Linux.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Install-Linux.tr)

Linux, systemd 240 veya sonrası ve root yetkisi gerekir. Kabul edilen mimariler `x86_64`, `aarch64`, `armv7l` ve `armv6l` biçimindedir. İki ağ arayüzünün düzenini İngilizce başlangıç kılavuzundan okuyun.

Önce betiği okuyun. Bu komut betiği gösterir, kurulum yapmaz:

```bash
curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh | less
```

İnceledikten sonra kurulumu çalıştırın:

```bash
sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"
```

Kurulum programı uygun dosyayı seçer ve yayımlanan sağlama toplamını denetler. Desteklenmeyen sistemde veya sağlama toplamı uyuşmazlığında durur. Güncellemek için aynı kurulum komutunu yeniden çalıştırın; kayıtlı ayarlar korunur. Elle kurulum ve kaynaktan derleme adımları İngilizce kılavuzdadır.

[İngilizce ayrıntılar](https://github.com/Iman/caspian/wiki/Installation#linux-and-raspberry-pi)

[Başlangıç (English)](https://github.com/Iman/caspian/wiki/Getting-Started)

[Caspian vikisi](https://github.com/Iman/caspian/wiki/Home.tr)

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)
