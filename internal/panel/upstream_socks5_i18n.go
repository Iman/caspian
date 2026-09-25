// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

func init() {
	messagesEN[Key("advanced.upstream.heading")] = "Upstream SOCKS5"
	messagesEN[Key("advanced.upstream.enable")] = "Use a SOCKS5 proxy as a front proxy for the tunnel server"
	messagesEN[Key("advanced.upstream.address")] = "SOCKS5 server address"
	messagesEN[Key("advanced.upstream.port")] = "SOCKS5 server port"
	messagesEN[Key("advanced.upstream.username")] = "Username (optional)"
	messagesEN[Key("advanced.upstream.password")] = "Password (optional)"
	messagesEN[Key("advanced.upstream.passwordset")] = "A password is already stored. Leave this field blank to keep it."
	messagesEN[Key("advanced.upstream.hint")] = "Caspian first connects to this SOCKS5 server, then reaches your configured tunnel server through it. Client traffic still uses your selected tunnel. Clear both credential fields to use no authentication."
	messagesEN[Key("advanced.upstream.bad")] = "Enter a SOCKS5 server address and a port."
	messagesEN[Key("advanced.upstream.portbad")] = "The SOCKS5 port must be a number from 1 to 65535."
	messagesEN[Key("advanced.upstream.authbad")] = "Enter both the SOCKS5 username and password, or leave both empty."

	messagesFA[Key("advanced.upstream.heading")] = "پراکسی بالادستی SOCKS5"
	messagesFA[Key("advanced.upstream.enable")] = "استفاده از یک پراکسی SOCKS5 به‌عنوان پراکسی واسط برای سرور تونل"
	messagesFA[Key("advanced.upstream.address")] = "نشانی سرور SOCKS5"
	messagesFA[Key("advanced.upstream.port")] = "پورت سرور SOCKS5"
	messagesFA[Key("advanced.upstream.username")] = "نام کاربری (اختیاری)"
	messagesFA[Key("advanced.upstream.password")] = "رمز عبور (اختیاری)"
	messagesFA[Key("advanced.upstream.passwordset")] = "رمز عبور قبلی ذخیره شده است. برای نگه داشتن آن این کادر را خالی بگذارید."
	messagesFA[Key("advanced.upstream.hint")] = "کاسپین ابتدا به این سرور SOCKS5 وصل می‌شود و سپس از طریق آن به سرور تونل تنظیم‌شده می‌رسد. ترافیک کاربران همچنان از تونل انتخاب‌شده عبور می‌کند. برای استفاده بدون احراز هویت، هر دو کادر نام کاربری و رمز عبور را خالی کنید."
	messagesFA[Key("advanced.upstream.bad")] = "نشانی و پورت سرور SOCKS5 را وارد کنید."
	messagesFA[Key("advanced.upstream.portbad")] = "پورت SOCKS5 باید عددی بین 1 تا 65535 باشد."
	messagesFA[Key("advanced.upstream.authbad")] = "نام کاربری و رمز عبور SOCKS5 را هر دو وارد کنید، یا هر دو را خالی بگذارید."
}
