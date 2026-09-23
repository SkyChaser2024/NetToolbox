package settings

func DefaultProfile() Profile {
	return Profile{RetryDelayMs: 2000, RememberPassword: true}
}

func DefaultDiagnostics() DiagnosticSettings {
	return DiagnosticSettings{
		LatencyTargets: []LatencyTarget{
			{ID: "douyin", Name: "字节抖音", URL: "https://www.douyin.com/", Region: "国内"},
			{ID: "bilibili", Name: "Bilibili", URL: "https://www.bilibili.com/", Region: "国内"},
			{ID: "wechat", Name: "腾讯微信", URL: "https://weixin.qq.com/", Region: "国内"},
			{ID: "taobao", Name: "阿里淘宝", URL: "https://www.taobao.com/", Region: "国内"},
			{ID: "github", Name: "GitHub", URL: "https://github.com/", Region: "国际"},
			{ID: "telegram", Name: "Telegram", URL: "https://telegram.org/", Region: "国际"},
			{ID: "x", Name: "X.com", URL: "https://x.com/", Region: "国际"},
			{ID: "youtube", Name: "YouTube", URL: "https://www.youtube.com/", Region: "国际"},
		},
		NATServers: []string{
			"stun.miwifi.com:3478",
			"stun.hitv.com:3478",
			"stun.chat.bilibili.com:3478",
			"stun.cloudflare.com:3478",
		},
		IPv4Endpoints: []string{
			"https://4.ipw.cn",
			"https://api-ipv4.ip.sb/ip",
			"https://myip.ipip.net",
			"https://ipv4.icanhazip.com",
		},
		IPv6Endpoints: []string{
			"https://6.ipw.cn",
			"https://api-ipv6.ip.sb/ip",
			"https://ipv6.icanhazip.com",
		},
		IPv6Sites: []string{
			"https://www.qq.com",
			"https://www.baidu.com",
			"https://www.taobao.com",
			"https://www.jd.com",
		},
		AAAADomain: "www.qq.com",
	}
}

func DefaultSystemPreferences() SystemPreferences {
	return SystemPreferences{PriorityMode: "automatic", CloseToTray: true}
}
