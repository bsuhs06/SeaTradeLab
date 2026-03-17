---
categories: osint maritime geopolitics
date: 2026-03-16
layout: post
title: "A Bit of Chaos"
reading_time: 4
tags: [osint, maritime, strait-of-hormuz, geopolitics]
---

# A Bit of Chaos

The Strait of Hormuz has always been a critical chokepoint — roughly a fifth of the world's oil passes through it on any given day. It has become even more significant over the past few weeks due to the current conflict in Iran. I unfortunately missed data collection at the beginning of the conflict; however, there are some definite trends emerging from the data I've gathered over the past several days.

![Strait of Hormuz]({{ '/assets/images/posts/StraitOfHormuz.png' | relative_url }})

The first thing you might notice in the image is a cluster of numbers appearing in the middle of a landmass. This is indicative of AIS spoofing — something I've been observing fairly frequently over the past days while monitoring this area. The occurrence of spoofed positions has increased noticeably since the first vessels were damaged.

There is also a large number of ships anchoring in various areas around the Persian Gulf, with what appears to be a significant concentration near the UAE. While some degree of anchorage is expected for vessels awaiting loading or offloading, the current numbers are likely well above typical levels. Other vessels in the Persian Gulf have been turning off their AIS transponders and anchoring in place in other areas as well.

There have also been some notable changes in how vessels are using their AIS data — specifically, altering their broadcast information to convey that they are not a party to the ongoing conflict.

I've also observed signs of a few vessels disabling AIS to transit the strait. In the screengrab above, these are represented by an amber line that crosses over land. The number of ships doing this has been extremely limited, especially over the past 72 hours.

I still have limited data on the region due to the sparseness of open-source AIS coverage in the area, and these are some of the more surface-level observations I've been able to make so far. The current conflict, while analytically significant, does not help with accurate collection due to the high amount of spoofing in the region and the fact that most AIS signals are being turned off. Satellite imagery could be incorporated to better understand the congestion.

As this conflict continues and the closure of the strait persists, oil and gas prices can be expected to rise. Natural gas will be a particularly significant factor, given that roughly [half of all charterable LNG carriers are currently trapped in the Gulf](https://www.wsj.com/livecoverage/us-israel-iran-war-2026/card/half-of-all-lng-carriers-trapped-in-the-gulf-brokers-say-huDGf773gO12FkvHLd6V). Another sector that stands to be heavily impacted is fertilizers. Nitrates are commonly produced alongside natural gas, and a shortage of fertilizers could put strain on future food production. This pressure is compounded by the fact that Russia was a major global producer of fertilizer before sanctions were imposed — some of which have since been eased on certain products. A supply shortage could also raise safety risks at production sites, as increased demand combined with deferred maintenance creates more volatile operating conditions. Disrupted supply chains can also lead to uneven distribution — shortages in some regions while excess inventory accumulates unsafely in others. Nitrate production has always carried inherent risk, and poorly managed stockpiles have historically proven hazardous — Beirut being a stark example.

That said, I'd consider large-scale stockpile accumulation in the Middle East unlikely at this stage. Much of the production infrastructure has likely been shut down due to the hostilities, and not all of it gracefully. It will also take time to bring everything back online and to repair damage inflicted by drones and missiles.

I'll continue monitoring and will share more detailed analysis once I've had time to build a proper baseline for comparison. For now, this is mostly observational — documenting what the data looks like so I can measure changes going forward.

---

*All observations are based on publicly available AIS data and are presented for research purposes only.*
