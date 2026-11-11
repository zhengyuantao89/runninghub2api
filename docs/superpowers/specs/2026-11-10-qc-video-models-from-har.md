# QC Video models from HAR `runninghub11.2image2.devtools`

Date: 2026-11-10

## Creation modes (VIDEO)

- `MULTIMODAL_REFERENCE` — Multimodal Reference (bindings=17)
- `SUBJECT_REFERENCE` — Subject Reference (bindings=13)
- `FREE_CREATION` — Free Creation (bindings=70)
- `FIRST_LAST_FRAME` — First & Last Frame (bindings=16)

Screenshot mapping:
- 多模态参考 = `MULTIMODAL_REFERENCE`
- 自由创作 = `FREE_CREATION`
- 首尾帧 = `FIRST_LAST_FRAME`
- 主体参考 = `SUBJECT_REFERENCE`
- 视频编辑 = usually `VIDEO_EDIT` category (separate from VIDEO creation-modes in this HAR)

## VIDEO model catalog (87 entries)

| # | Name | Group | bindingId | skuId | modes | billing(from fee-preview) | params seen |
|---|------|-------|-----------|-------|-------|---------------------------|-------------|
| 1 | seedance2.5-global/多模态视频 Token nsfw | Seedance 2.5 | `2084840518132973570` | `2133100000000700025` | MULTIMODAL_REFERENCE | paid_seen | bitrateMode,conversionSlots,creationMode,duration,generateAudio,imageUrls,omniReferenceTaskType,outputFormat,prompt,ratio,realPersonMode,resolution,returnLastFrame,seed,watermark |
| 2 | seedance2.5-global/图生视频 Token nsfw | Seedance 2.5 | `2084840595538853890` | `2133100000000700024` | FREE_CREATION | paid_seen | bitrateMode,conversionSlots,creationMode,duration,firstFrameUrl,generateAudio,outputFormat,prompt,ratio,realPersonMode,resolution,returnLastFrame,seed,watermark |
| 3 | seedance2.5-global/文生视频 Token nsfw | Seedance 2.5 | `2084840781472350210` | `2133100000000700023` | FREE_CREATION | unknown | - |
| 4 | MiniMax-H3-Max 图生视频（首尾帧） | MiniMax H3 Max | `2094323821554737153` | `2133100000000504205` | FREE_CREATION,FIRST_LAST_FRAME | paid_seen | creationMode,duration,firstFrameUrl,prompt,resolution |
| 5 | MiniMax-H3-Max 文生视频 | MiniMax H3 Max | `2094323902341226498` | `2133100000000504204` | FREE_CREATION | unknown | - |
| 6 | MiniMax-H3-Max-Turbo 图生视频（首尾帧） | MiniMax H3 Max Turbo | `2095692017767596034` | `2133100000000504402` | FIRST_LAST_FRAME | paid_seen | creationMode,duration,firstFrameUrl,prompt,resolution |
| 7 | MiniMax-H3-Max-Turbo 文生视频 | MiniMax H3 Max Turbo | `2095692103360757761` | `2133100000000504401` | FREE_CREATION | unknown | - |
| 8 | Minimax H3 RH Enhanced Multi-Reference | Minimax H3-RH Enhanced | `2103076210080342018` | `2102602670923161601` | MULTIMODAL_REFERENCE | free_seen | aspectRatio,audioMode,creationMode,duration,prompt,refImage1,resolution |
| 9 | Minimax H3 RH Enhanced Image-to-Video | Minimax H3-RH Enhanced | `2103076348802752513` | `2102602472381587457` | FREE_CREATION | free_seen | aspectRatio,audioMode,creationMode,duration,firstFrameUrl,prompt,resolution |
| 10 | Minimax H3 RH Enhanced Text-to-Video | Minimax H3-RH Enhanced | `2103076432063881217` | `2102602278114009090` | FREE_CREATION | unknown | - |
| 11 | vidu/q4-preview/image-to-video | Vidu Q4 Preview | `2108107301677756418` | `2133100000000800442` | FREE_CREATION | unknown | - |
| 12 | vidu/q4-preview/reference-to-video | Vidu Q4 Preview | `2108107457634562050` | `2133100000000800443` | MULTIMODAL_REFERENCE | unknown | - |
| 13 | 可灵文生视频4.0 | Kling 4.0 | `2113598112913756162` | `2062210220000000401` | FREE_CREATION | unknown | - |
| 14 | 可灵全能视频4.0 | Kling 4.0 | `2113598302286581762` | `2062210220000000403` | MULTIMODAL_REFERENCE | unknown | - |
| 15 | 可灵 4.0 主体创建 | Kling 4.0 | `2113598387737137154` | `2062210220000000411` | SUBJECT_REFERENCE | unknown | - |
| 16 | 可灵图生视频4.0 | Kling 4.0 | `2113598924226367490` | `2062210220000000402` | FREE_CREATION | unknown | - |
| 17 | 万相3.0-参考生视频-nsfw | WAN 3.0 | `2091710340359839745` | `2133100000000700019` | MULTIMODAL_REFERENCE | unknown | - |
| 18 | 万相3.0-图生视频-nsfw | WAN 3.0 | `2091710518592593921` | `2133100000000700018` | FREE_CREATION | unknown | - |
| 19 | 万相2.7-spicy-图生视频-nsfw | WAN2.7-Spicy | `2076590177377660930` | `2054816782671446017` | FREE_CREATION | unknown | - |
| 20 | gemini-omni-1.1-flash/image-to-video/stable | Gmini Omni 1.1 Flash | `2094315084777562113` | `2133100000000700037` | FREE_CREATION | unknown | - |
| 21 | gemini-omni-1.1-flash/reference-to-video/stable | Gmini Omni 1.1 Flash | `2094315174099460097` | `2133100000000700036` | FREE_CREATION | unknown | - |
| 22 | gemini-omni-1.1-flash/text-to-video/stable | Gmini Omni 1.1 Flash | `2094315288809480194` | `2133100000000700035` | FREE_CREATION | unknown | - |
| 23 | seedance2.0/多模态视频 | Seedance2.0 | `2046057458784591873` | `2034917373414539277` | MULTIMODAL_REFERENCE | unknown | - |
| 24 | seedance2.0/图生视频 | Seedance2.0 | `2046586338670891010` | `2034917373414539275` | FIRST_LAST_FRAME | unknown | - |
| 25 | 万相3.0高速版-参考生视频-nsfw | WAN 3.0-Prime | `2091817039544508418` | `2133100000000700034` | MULTIMODAL_REFERENCE | unknown | - |
| 26 | 万相3.0高速版-图生视频-nsfw | WAN 3.0-Prime | `2091817133090070530` | `2133100000000700033` | FREE_CREATION | unknown | - |
| 27 | MiniMax-H3 多模态参考生视频 | Hailuo H3 | `2083002667300921345` | `2133100000000504201` | MULTIMODAL_REFERENCE | unknown | - |
| 28 | MiniMax-H3 图生视频（首尾帧） | Hailuo H3 | `2082876967445176321` | `2133100000000504203` | - | unknown | - |
| 29 | MiniMax-H3 文生视频 | Hailuo H3 | `2082877522762637313` | `2133100000000504202` | FREE_CREATION | unknown | - |
| 30 | seedance2.0-global-fast/多模态视频-nsfw | Seedance2.0-Global-Fast | `2076612927920091138` | `2042415417236176910` | MULTIMODAL_REFERENCE | unknown | - |
| 31 | seedance2.0-global-fast/图生视频-nsfw | Seedance2.0-Global-Fast | `2076613175027511297` | `2042415417236176909` | FREE_CREATION | unknown | - |
| 32 | seedance2.0-global-fast/文生视频-nsfw | Seedance2.0-Global-Fast | `2076613294913302530` | `2042415417236176908` | FREE_CREATION | unknown | - |
| 33 | seedance2.0-global/多模态视频-nsfw | Seedance2.0-Global | `2076614432429518849` | `2042415417236176907` | MULTIMODAL_REFERENCE | unknown | - |
| 34 | seedance2.0-global/图生视频-nsfw | Seedance2.0-Global | `2076614949402652673` | `2042415417236176906` | FREE_CREATION | unknown | - |
| 35 | seedance2.0-global/文生视频-nsfw | Seedance2.0-Global | `2076615036702896130` | `2042415417236176905` | FREE_CREATION | unknown | - |
| 36 | seedance2.0-global-Mini/多模态视频-nsfw | Seedance2.0-Global-Mini | `2076617226553208834` | `2133000000000504052` | MULTIMODAL_REFERENCE,FREE_CREATION | unknown | - |
| 37 | seedance2.0-global-Mini/文生视频-nsfw | Seedance2.0-Global-Mini | `2076617555474722818` | `2133000000000504050` | FREE_CREATION | unknown | - |
| 38 | seedance2.0-global-Mini/图生视频-nsfw | Seedance2.0-Global-Mini | `2076627883457589249` | `2133000000000504051` | FREE_CREATION | unknown | - |
| 39 | seedance2.0-Mini/多模态视频 | Seedance2.0-Mini | `2068890464809152513` | `2133000000000504043` | MULTIMODAL_REFERENCE | unknown | - |
| 40 | seedance2.0-Mini/图生视频 | Seedance2.0-Mini | `2068899465194139649` | `2133000000000504042` | FREE_CREATION | unknown | - |
| 41 | seedance2.0-Mini/文生视频 | Seedance2.0-Mini | `2068899910188822530` | `2133000000000504041` | FREE_CREATION | unknown | - |
| 42 | seedance2.0-Fast/多模态视频 | Seedance2.0-Fast | `2044238663652761602` | `2034917373414539278` | MULTIMODAL_REFERENCE | unknown | - |
| 43 | seedance2.0-Fast/图生视频 | Seedance2.0-Fast | `2046586338670891011` | `2034917373414539276` | FIRST_LAST_FRAME | unknown | - |
| 44 | happyhorse-1.1/reference-to-video-nsfw | HappyHorse 1.1 | `2069058513981960193` | `2132764885651525693` | MULTIMODAL_REFERENCE | unknown | - |
| 45 | happyhorse-1.1/text-to-video-nsfw | HappyHorse 1.1 | `2069053574610186242` | `2132764885651525691` | FREE_CREATION | unknown | - |
| 46 | happyhorse-1.1/image-to-video-nsfw | HappyHorse 1.1 | `2069053836833878018` | `2132764885651525692` | FREE_CREATION | unknown | - |
| 47 | 可灵图生视频v3-turbo-std | KlingV3-Turbo-Std | `2071542923209773057` | `2133000000000504057` | FREE_CREATION | unknown | - |
| 48 | 可灵文生视频v3-turbo-std | KlingV3-Turbo-Std | `2071579059516915713` | `2069321552501190658` | FREE_CREATION | unknown | - |
| 49 | 可灵文生视频v3-turbo-pro | Klingv3-Turbo-Pro | `2071543617299976194` | `2133000000000504056` | FREE_CREATION | unknown | - |
| 50 | 可灵图生视频v3-turbo-pro | Klingv3-Turbo-Pro | `2071573824715145217` | `2133000000000504058` | FREE_CREATION | unknown | - |
| 51 | xai/grok-imagine-video/image-to-video/stable | Grok Imagine 1.5-Official | `2044263978663960578` | `2028306154142318593` | FREE_CREATION | unknown | - |
| 52 | xai/grok-imagine-v1.5/image-to-video/economy | Grok Imagine 1.5-Low-Cost | `2044264799178874881` | `2019380112598044674` | FREE_CREATION | unknown | - |
| 53 | gemini-omni-flash/image-to-video/economy | Gemini Omni Flash | `2058148323887706114` | `2132764885651525659` | FREE_CREATION | unknown | - |
| 54 | gemini-omni-flash/text-to-video/economy | Gemini Omni Flash | `2058150582226808834` | `2132764885651525658` | FREE_CREATION | unknown | - |
| 55 | 可灵图生视频v3-4k | Kling3.0-4k | `2047886748354351105` | `2047563851324841986` | FREE_CREATION | unknown | - |
| 56 | 可灵文生视频v3-4k | Kling3.0-4k | `2047886673163063298` | `2047563851324841987` | FREE_CREATION | unknown | - |
| 57 | 可灵参考生视频o3-4k | KlingO3-4k | `2047886370279788546` | `2047563851324841990` | SUBJECT_REFERENCE | paid_seen | aspectRatio,creationMode,duration,imageUrls,keepOriginalSound,prompt,shotType,sound |
| 58 | 可灵图生视频o3-4k | KlingO3-4k | `2047886446062473218` | `2047563851324841989` | FIRST_LAST_FRAME | unknown | - |
| 59 | 可灵文生视频o3-4k | KlingO3-4k | `2047886511770439682` | `2047563851324841988` | FREE_CREATION | unknown | - |
| 60 | kling-video-o3-pro/reference-to-video | Kling O3-Pro | `2043995490221330433` | `2019638479950254081` | SUBJECT_REFERENCE | unknown | - |
| 61 | 可灵文生视频o3-pro | Kling O3-Pro | `2044000337133867010` | `2019620511153459202` | FREE_CREATION | unknown | - |
| 62 | kling-video-o3-std/reference-to-video | Kling O3-Std | `2044418820103507970` | `2019634451799412737` | SUBJECT_REFERENCE | unknown | - |
| 63 | 可灵文生视频o3-std | Kling O3-Std | `2044257834381893634` | `2019608799960436737` | FREE_CREATION | unknown | - |
| 64 | 可灵参考生视频o1 | Kling O1 | `2043969652037857281` | `2012067220412493827` | SUBJECT_REFERENCE | paid_seen | aspectRatio,creationMode,duration,imageUrls,keepOriginalSound,mode,prompt |
| 65 | 可灵首尾帧生视频o1 | Kling O1 | `2044307181475749889` | `2012030892408893442` | FIRST_LAST_FRAME | unknown | - |
| 66 | 可灵图生视频o1 | Kling O1 | `2044312939156234242` | `2012004604507910146` | FREE_CREATION | unknown | - |
| 67 | 万相2.7-参考生视频 | WAN 2.7 | `2043968766691586049` | `2039648613636050945` | SUBJECT_REFERENCE | paid_seen | aspectRatio,creationMode,duration,imageUrls,prompt,resolution |
| 68 | 万相2.7-图生视频 | WAN 2.7 | `2043978889392590849` | `2039618329897144322` | FIRST_LAST_FRAME | unknown | - |
| 69 | 万相2.7-文生视频 | WAN 2.7 | `2044418820103507976` | `2039544460993695745` | FREE_CREATION | unknown | - |
| 70 | PixVerse V6 图生视频 | PixVerseV6 | `2043984892905426945` | `2041408069159936003` | FREE_CREATION | unknown | - |
| 71 | PixVerse V6 文生视频 | PixVerseV6 | `2044418820103507975` | `2041408069159936002` | FREE_CREATION | unknown | - |
| 72 | happyhorse-1.0/reference-to-video-nsfw | HappyHorse | `2048721908209594370` | `2048623940504719363` | MULTIMODAL_REFERENCE | unknown | - |
| 73 | happyhorse-1.0/image-to-video-nsfw | HappyHorse | `2048720945017040897` | `2046514150500524037` | FREE_CREATION | unknown | - |
| 74 | happyhorse-1.0/text-to-video-nsfw | HappyHorse | `2048699038968385537` | `2046514150500524036` | FREE_CREATION | unknown | - |
| 75 | google/veo3.1-pro/start-end-to-video/economy | Veo3.1-Pro-Low-Cost | `2044299767141654530` | `2018599147311271938` | FIRST_LAST_FRAME | paid_seen | aspectRatio,creationMode,duration,firstFrameUrl,prompt,resolution |
| 76 | google/veo3.1-pro/text-to-video/economy | Veo3.1-Pro-Low-Cost | `2044303906835816450` | `2005884653783007234` | FREE_CREATION | unknown | - |
| 77 | google/veo3.1-fast/reference-to-video/stable | Veo3.1-Fast-Official | `2056305155177201666` | `2054086928526929953` | SUBJECT_REFERENCE | paid_seen | aspectRatio,creationMode,generateAudio,imageUrls,prompt,resolution |
| 78 | google/veo3.1-fast/image-to-video/stable | Veo3.1-Fast-Official | `2044300909317742593` | `2022225870330286082` | FIRST_LAST_FRAME | paid_seen | aspectRatio,creationMode,duration,generateAudio,imageUrl,prompt,resolution |
| 79 | google/veo3.1-fast/text-to-video/stable | Veo3.1-Fast-Official | `2044304543057207297` | `2022220635650150401` | FREE_CREATION | unknown | - |
| 80 | google/veo3.1-fast/start-end-to-video/economy | Veo3.1-Fast-Low-Cost | `2044261714113093634` | `2016052223404204034` | FIRST_LAST_FRAME | unknown | - |
| 81 | google/veo3.1-fast/text-to-video/economy | Veo3.1-Fast-Low-Cost | `2044302636406300674` | `2005884261993070594` | FREE_CREATION | unknown | - |
| 82 | 海螺-2.3-图生视频-pro | Hailuo2.3 Pro | `2049089576389341185` | `2011758831593648131` | FREE_CREATION | unknown | - |
| 83 | 海螺-2.3-文生视频-pro | Hailuo2.3 Pro | `2049100357059301378` | `2011999330699112450` | FREE_CREATION | unknown | - |
| 84 | Vidu-首尾帧生视频-q3-pro | ViduQ3-Pro | `2044418820103507969` | `2019687963136692226` | FIRST_LAST_FRAME | paid_seen | audio,creationMode,duration,firstImageUrl,movementAmplitude,prompt,resolution |
| 85 | Vidu-文生视频-q3-pro | ViduQ3-Pro | `2044418820103507978` | `2017148354741735426` | FREE_CREATION | unknown | - |
| 86 | Vidu-首尾帧生视频-q3-pro-fast | ViduQ3-Pro-Fast | `2043982835939053570` | `2034917373414539268` | FIRST_LAST_FRAME | unknown | - |
| 87 | Vidu-文生视频-q3-pro-fast | ViduQ3-Pro-Fast | `2044418820103507977` | `2034917373414539266` | FREE_CREATION | unknown | - |

## Per-model details

### seedance2.5-global/多模态视频 Token nsfw

- group: Seedance 2.5
- bindingId: `2084840518132973570`
- skuId: `2133100000000700025`
- modes: MULTIMODAL_REFERENCE
- billing hint: `paid_seen`
- params seen in fee-preview only: bitrateMode, conversionSlots, creationMode, duration, generateAudio, imageUrls, omniReferenceTaskType, outputFormat, prompt, ratio, realPersonMode, resolution, returnLastFrame, seed, watermark
- fee sample: free=False points=988720 cash=1.454 quota=0 params={'bitrateMode': 'standard', 'conversionSlots': ['all'], 'creationMode': 'multimodal', 'duration': '5', 'generateAudio': True, 'imageUrls': ['https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png'], 'omniReferenceTaskType': 'auto', 'outputFormat': 'mp4', 'ratio': 'adaptive', 'realPersonMode': True, 'resolution': '720p', 'returnLastFrame': False, 'seed': -1, 'watermark': False}
- fee sample: free=False points=988720 cash=1.454 quota=0 params={'bitrateMode': 'standard', 'conversionSlots': ['all'], 'creationMode': 'multimodal', 'duration': '5', 'generateAudio': True, 'imageUrls': ['https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png'], 'omniReferenceTaskType': 'auto', 'outputFormat': 'mp4', 'ratio': 'adaptive', 'realPersonMode': True, 'resolution': '720p', 'returnLastFrame': False, 'seed': -1, 'watermark': False}

### seedance2.5-global/图生视频 Token nsfw

- group: Seedance 2.5
- bindingId: `2084840595538853890`
- skuId: `2133100000000700024`
- modes: FREE_CREATION
- billing hint: `paid_seen`
- params seen in fee-preview only: bitrateMode, conversionSlots, creationMode, duration, firstFrameUrl, generateAudio, outputFormat, prompt, ratio, realPersonMode, resolution, returnLastFrame, seed, watermark
- fee sample: free=False points=988720 cash=1.454 quota=0 params={'bitrateMode': 'standard', 'conversionSlots': ['all'], 'creationMode': 'freeCreation', 'duration': '5', 'firstFrameUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'generateAudio': True, 'outputFormat': 'mp4', 'ratio': 'adaptive', 'realPersonMode': True, 'resolution': '720p', 'returnLastFrame': False, 'seed': -1, 'watermark': False}
- fee sample: free=False points=988720 cash=1.454 quota=0 params={'bitrateMode': 'standard', 'conversionSlots': ['all'], 'creationMode': 'freeCreation', 'duration': '5', 'firstFrameUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'generateAudio': True, 'outputFormat': 'mp4', 'ratio': 'adaptive', 'realPersonMode': True, 'resolution': '720p', 'returnLastFrame': False, 'seed': -1, 'watermark': False}

### seedance2.5-global/文生视频 Token nsfw

- group: Seedance 2.5
- bindingId: `2084840781472350210`
- skuId: `2133100000000700023`
- modes: FREE_CREATION
- billing hint: `unknown`

### MiniMax-H3-Max 图生视频（首尾帧）

- group: MiniMax H3 Max
- bindingId: `2094323821554737153`
- skuId: `2133100000000504205`
- modes: FREE_CREATION, FIRST_LAST_FRAME
- billing hint: `paid_seen`
- params seen in fee-preview only: creationMode, duration, firstFrameUrl, prompt, resolution
- fee sample: free=False points=340000 cash=0.5 quota=0 params={'creationMode': 'firstLastFrame', 'duration': '5', 'firstFrameUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'resolution': '768P'}
- fee sample: free=False points=340000 cash=0.5 quota=0 params={'creationMode': 'firstLastFrame', 'duration': '5', 'firstFrameUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'resolution': '768P'}

### MiniMax-H3-Max 文生视频

- group: MiniMax H3 Max
- bindingId: `2094323902341226498`
- skuId: `2133100000000504204`
- modes: FREE_CREATION
- billing hint: `unknown`

### MiniMax-H3-Max-Turbo 图生视频（首尾帧）

- group: MiniMax H3 Max Turbo
- bindingId: `2095692017767596034`
- skuId: `2133100000000504402`
- modes: FIRST_LAST_FRAME
- billing hint: `paid_seen`
- params seen in fee-preview only: creationMode, duration, firstFrameUrl, prompt, resolution
- fee sample: free=False points=170000 cash=0.25 quota=0 params={'creationMode': 'firstLastFrame', 'duration': '5', 'firstFrameUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'resolution': '768p'}
- fee sample: free=False points=170000 cash=0.25 quota=0 params={'creationMode': 'freeCreation', 'duration': '5', 'firstFrameUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'resolution': '768p'}

### MiniMax-H3-Max-Turbo 文生视频

- group: MiniMax H3 Max Turbo
- bindingId: `2095692103360757761`
- skuId: `2133100000000504401`
- modes: FREE_CREATION
- billing hint: `unknown`

### Minimax H3 RH Enhanced Multi-Reference

- group: Minimax H3-RH Enhanced
- bindingId: `2103076210080342018`
- skuId: `2102602670923161601`
- modes: MULTIMODAL_REFERENCE
- billing hint: `free_seen`
- params seen in fee-preview only: aspectRatio, audioMode, creationMode, duration, prompt, refImage1, resolution
- fee sample: free=True points=None cash=0 quota=1 params={'aspectRatio': '1:1 (Square)', 'audioMode': 'native', 'creationMode': 'multimodal', 'duration': 5, 'refImage1': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'resolution': '1080p'}
- fee sample: free=True points=None cash=0 quota=1 params={'aspectRatio': '1:1 (Square)', 'audioMode': 'native', 'creationMode': 'multimodal', 'duration': 5, 'refImage1': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'resolution': '1080p'}

### Minimax H3 RH Enhanced Image-to-Video

- group: Minimax H3-RH Enhanced
- bindingId: `2103076348802752513`
- skuId: `2102602472381587457`
- modes: FREE_CREATION
- billing hint: `free_seen`
- params seen in fee-preview only: aspectRatio, audioMode, creationMode, duration, firstFrameUrl, prompt, resolution
- fee sample: free=True points=None cash=0 quota=1 params={'aspectRatio': '1:1 (Square)', 'audioMode': 'native', 'creationMode': 'freeCreation', 'duration': 5, 'firstFrameUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'resolution': '1080p'}

### Minimax H3 RH Enhanced Text-to-Video

- group: Minimax H3-RH Enhanced
- bindingId: `2103076432063881217`
- skuId: `2102602278114009090`
- modes: FREE_CREATION
- billing hint: `unknown`

### vidu/q4-preview/image-to-video

- group: Vidu Q4 Preview
- bindingId: `2108107301677756418`
- skuId: `2133100000000800442`
- modes: FREE_CREATION
- billing hint: `unknown`

### vidu/q4-preview/reference-to-video

- group: Vidu Q4 Preview
- bindingId: `2108107457634562050`
- skuId: `2133100000000800443`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### 可灵文生视频4.0

- group: Kling 4.0
- bindingId: `2113598112913756162`
- skuId: `2062210220000000401`
- modes: FREE_CREATION
- billing hint: `unknown`

### 可灵全能视频4.0

- group: Kling 4.0
- bindingId: `2113598302286581762`
- skuId: `2062210220000000403`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### 可灵 4.0 主体创建

- group: Kling 4.0
- bindingId: `2113598387737137154`
- skuId: `2062210220000000411`
- modes: SUBJECT_REFERENCE
- billing hint: `unknown`

### 可灵图生视频4.0

- group: Kling 4.0
- bindingId: `2113598924226367490`
- skuId: `2062210220000000402`
- modes: FREE_CREATION
- billing hint: `unknown`

### 万相3.0-参考生视频-nsfw

- group: WAN 3.0
- bindingId: `2091710340359839745`
- skuId: `2133100000000700019`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### 万相3.0-图生视频-nsfw

- group: WAN 3.0
- bindingId: `2091710518592593921`
- skuId: `2133100000000700018`
- modes: FREE_CREATION
- billing hint: `unknown`

### 万相2.7-spicy-图生视频-nsfw

- group: WAN2.7-Spicy
- bindingId: `2076590177377660930`
- skuId: `2054816782671446017`
- modes: FREE_CREATION
- billing hint: `unknown`

### gemini-omni-1.1-flash/image-to-video/stable

- group: Gmini Omni 1.1 Flash
- bindingId: `2094315084777562113`
- skuId: `2133100000000700037`
- modes: FREE_CREATION
- billing hint: `unknown`

### gemini-omni-1.1-flash/reference-to-video/stable

- group: Gmini Omni 1.1 Flash
- bindingId: `2094315174099460097`
- skuId: `2133100000000700036`
- modes: FREE_CREATION
- billing hint: `unknown`

### gemini-omni-1.1-flash/text-to-video/stable

- group: Gmini Omni 1.1 Flash
- bindingId: `2094315288809480194`
- skuId: `2133100000000700035`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0/多模态视频

- group: Seedance2.0
- bindingId: `2046057458784591873`
- skuId: `2034917373414539277`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### seedance2.0/图生视频

- group: Seedance2.0
- bindingId: `2046586338670891010`
- skuId: `2034917373414539275`
- modes: FIRST_LAST_FRAME
- billing hint: `unknown`

### 万相3.0高速版-参考生视频-nsfw

- group: WAN 3.0-Prime
- bindingId: `2091817039544508418`
- skuId: `2133100000000700034`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### 万相3.0高速版-图生视频-nsfw

- group: WAN 3.0-Prime
- bindingId: `2091817133090070530`
- skuId: `2133100000000700033`
- modes: FREE_CREATION
- billing hint: `unknown`

### MiniMax-H3 多模态参考生视频

- group: Hailuo H3
- bindingId: `2083002667300921345`
- skuId: `2133100000000504201`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### MiniMax-H3 图生视频（首尾帧）

- group: Hailuo H3
- bindingId: `2082876967445176321`
- skuId: `2133100000000504203`
- modes: -
- billing hint: `unknown`

### MiniMax-H3 文生视频

- group: Hailuo H3
- bindingId: `2082877522762637313`
- skuId: `2133100000000504202`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0-global-fast/多模态视频-nsfw

- group: Seedance2.0-Global-Fast
- bindingId: `2076612927920091138`
- skuId: `2042415417236176910`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### seedance2.0-global-fast/图生视频-nsfw

- group: Seedance2.0-Global-Fast
- bindingId: `2076613175027511297`
- skuId: `2042415417236176909`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0-global-fast/文生视频-nsfw

- group: Seedance2.0-Global-Fast
- bindingId: `2076613294913302530`
- skuId: `2042415417236176908`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0-global/多模态视频-nsfw

- group: Seedance2.0-Global
- bindingId: `2076614432429518849`
- skuId: `2042415417236176907`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### seedance2.0-global/图生视频-nsfw

- group: Seedance2.0-Global
- bindingId: `2076614949402652673`
- skuId: `2042415417236176906`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0-global/文生视频-nsfw

- group: Seedance2.0-Global
- bindingId: `2076615036702896130`
- skuId: `2042415417236176905`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0-global-Mini/多模态视频-nsfw

- group: Seedance2.0-Global-Mini
- bindingId: `2076617226553208834`
- skuId: `2133000000000504052`
- modes: MULTIMODAL_REFERENCE, FREE_CREATION
- billing hint: `unknown`

### seedance2.0-global-Mini/文生视频-nsfw

- group: Seedance2.0-Global-Mini
- bindingId: `2076617555474722818`
- skuId: `2133000000000504050`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0-global-Mini/图生视频-nsfw

- group: Seedance2.0-Global-Mini
- bindingId: `2076627883457589249`
- skuId: `2133000000000504051`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0-Mini/多模态视频

- group: Seedance2.0-Mini
- bindingId: `2068890464809152513`
- skuId: `2133000000000504043`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### seedance2.0-Mini/图生视频

- group: Seedance2.0-Mini
- bindingId: `2068899465194139649`
- skuId: `2133000000000504042`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0-Mini/文生视频

- group: Seedance2.0-Mini
- bindingId: `2068899910188822530`
- skuId: `2133000000000504041`
- modes: FREE_CREATION
- billing hint: `unknown`

### seedance2.0-Fast/多模态视频

- group: Seedance2.0-Fast
- bindingId: `2044238663652761602`
- skuId: `2034917373414539278`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### seedance2.0-Fast/图生视频

- group: Seedance2.0-Fast
- bindingId: `2046586338670891011`
- skuId: `2034917373414539276`
- modes: FIRST_LAST_FRAME
- billing hint: `unknown`

### happyhorse-1.1/reference-to-video-nsfw

- group: HappyHorse 1.1
- bindingId: `2069058513981960193`
- skuId: `2132764885651525693`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### happyhorse-1.1/text-to-video-nsfw

- group: HappyHorse 1.1
- bindingId: `2069053574610186242`
- skuId: `2132764885651525691`
- modes: FREE_CREATION
- billing hint: `unknown`

### happyhorse-1.1/image-to-video-nsfw

- group: HappyHorse 1.1
- bindingId: `2069053836833878018`
- skuId: `2132764885651525692`
- modes: FREE_CREATION
- billing hint: `unknown`

### 可灵图生视频v3-turbo-std

- group: KlingV3-Turbo-Std
- bindingId: `2071542923209773057`
- skuId: `2133000000000504057`
- modes: FREE_CREATION
- billing hint: `unknown`

### 可灵文生视频v3-turbo-std

- group: KlingV3-Turbo-Std
- bindingId: `2071579059516915713`
- skuId: `2069321552501190658`
- modes: FREE_CREATION
- billing hint: `unknown`

### 可灵文生视频v3-turbo-pro

- group: Klingv3-Turbo-Pro
- bindingId: `2071543617299976194`
- skuId: `2133000000000504056`
- modes: FREE_CREATION
- billing hint: `unknown`

### 可灵图生视频v3-turbo-pro

- group: Klingv3-Turbo-Pro
- bindingId: `2071573824715145217`
- skuId: `2133000000000504058`
- modes: FREE_CREATION
- billing hint: `unknown`

### xai/grok-imagine-video/image-to-video/stable

- group: Grok Imagine 1.5-Official
- bindingId: `2044263978663960578`
- skuId: `2028306154142318593`
- modes: FREE_CREATION
- billing hint: `unknown`

### xai/grok-imagine-v1.5/image-to-video/economy

- group: Grok Imagine 1.5-Low-Cost
- bindingId: `2044264799178874881`
- skuId: `2019380112598044674`
- modes: FREE_CREATION
- billing hint: `unknown`

### gemini-omni-flash/image-to-video/economy

- group: Gemini Omni Flash
- bindingId: `2058148323887706114`
- skuId: `2132764885651525659`
- modes: FREE_CREATION
- billing hint: `unknown`

### gemini-omni-flash/text-to-video/economy

- group: Gemini Omni Flash
- bindingId: `2058150582226808834`
- skuId: `2132764885651525658`
- modes: FREE_CREATION
- billing hint: `unknown`

### 可灵图生视频v3-4k

- group: Kling3.0-4k
- bindingId: `2047886748354351105`
- skuId: `2047563851324841986`
- modes: FREE_CREATION
- billing hint: `unknown`

### 可灵文生视频v3-4k

- group: Kling3.0-4k
- bindingId: `2047886673163063298`
- skuId: `2047563851324841987`
- modes: FREE_CREATION
- billing hint: `unknown`

### 可灵参考生视频o3-4k

- group: KlingO3-4k
- bindingId: `2047886370279788546`
- skuId: `2047563851324841990`
- modes: SUBJECT_REFERENCE
- billing hint: `paid_seen`
- params seen in fee-preview only: aspectRatio, creationMode, duration, imageUrls, keepOriginalSound, prompt, shotType, sound
- fee sample: free=False points=2067200 cash=3.04 quota=0 params={'aspectRatio': '16:9', 'creationMode': 'subjectRef', 'duration': '8', 'imageUrls': ['https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png'], 'keepOriginalSound': 'true', 'shotType': 'customize', 'sound': 'false'}
- fee sample: free=False points=1292000 cash=1.9 quota=0 params={'aspectRatio': '16:9', 'creationMode': 'subjectRef', 'duration': '5', 'imageUrls': ['https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png'], 'keepOriginalSound': 'true', 'shotType': 'customize', 'sound': 'false'}

### 可灵图生视频o3-4k

- group: KlingO3-4k
- bindingId: `2047886446062473218`
- skuId: `2047563851324841989`
- modes: FIRST_LAST_FRAME
- billing hint: `unknown`

### 可灵文生视频o3-4k

- group: KlingO3-4k
- bindingId: `2047886511770439682`
- skuId: `2047563851324841988`
- modes: FREE_CREATION
- billing hint: `unknown`

### kling-video-o3-pro/reference-to-video

- group: Kling O3-Pro
- bindingId: `2043995490221330433`
- skuId: `2019638479950254081`
- modes: SUBJECT_REFERENCE
- billing hint: `unknown`

### 可灵文生视频o3-pro

- group: Kling O3-Pro
- bindingId: `2044000337133867010`
- skuId: `2019620511153459202`
- modes: FREE_CREATION
- billing hint: `unknown`

### kling-video-o3-std/reference-to-video

- group: Kling O3-Std
- bindingId: `2044418820103507970`
- skuId: `2019634451799412737`
- modes: SUBJECT_REFERENCE
- billing hint: `unknown`

### 可灵文生视频o3-std

- group: Kling O3-Std
- bindingId: `2044257834381893634`
- skuId: `2019608799960436737`
- modes: FREE_CREATION
- billing hint: `unknown`

### 可灵参考生视频o1

- group: Kling O1
- bindingId: `2043969652037857281`
- skuId: `2012067220412493827`
- modes: SUBJECT_REFERENCE
- billing hint: `paid_seen`
- params seen in fee-preview only: aspectRatio, creationMode, duration, imageUrls, keepOriginalSound, mode, prompt
- fee sample: free=False points=360400 cash=0.53 quota=0 params={'aspectRatio': '16:9', 'creationMode': 'subjectRef', 'duration': '5', 'imageUrls': ['https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png'], 'keepOriginalSound': True, 'mode': 'std'}

### 可灵首尾帧生视频o1

- group: Kling O1
- bindingId: `2044307181475749889`
- skuId: `2012030892408893442`
- modes: FIRST_LAST_FRAME
- billing hint: `unknown`

### 可灵图生视频o1

- group: Kling O1
- bindingId: `2044312939156234242`
- skuId: `2012004604507910146`
- modes: FREE_CREATION
- billing hint: `unknown`

### 万相2.7-参考生视频

- group: WAN 2.7
- bindingId: `2043968766691586049`
- skuId: `2039648613636050945`
- modes: SUBJECT_REFERENCE
- billing hint: `paid_seen`
- params seen in fee-preview only: aspectRatio, creationMode, duration, imageUrls, prompt, resolution
- fee sample: free=False points=408000 cash=0.6 quota=0 params={'aspectRatio': '16:9', 'creationMode': 'subjectRef', 'duration': '5', 'imageUrls': ['https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png'], 'resolution': '1080P'}

### 万相2.7-图生视频

- group: WAN 2.7
- bindingId: `2043978889392590849`
- skuId: `2039618329897144322`
- modes: FIRST_LAST_FRAME
- billing hint: `unknown`

### 万相2.7-文生视频

- group: WAN 2.7
- bindingId: `2044418820103507976`
- skuId: `2039544460993695745`
- modes: FREE_CREATION
- billing hint: `unknown`

### PixVerse V6 图生视频

- group: PixVerseV6
- bindingId: `2043984892905426945`
- skuId: `2041408069159936003`
- modes: FREE_CREATION
- billing hint: `unknown`

### PixVerse V6 文生视频

- group: PixVerseV6
- bindingId: `2044418820103507975`
- skuId: `2041408069159936002`
- modes: FREE_CREATION
- billing hint: `unknown`

### happyhorse-1.0/reference-to-video-nsfw

- group: HappyHorse
- bindingId: `2048721908209594370`
- skuId: `2048623940504719363`
- modes: MULTIMODAL_REFERENCE
- billing hint: `unknown`

### happyhorse-1.0/image-to-video-nsfw

- group: HappyHorse
- bindingId: `2048720945017040897`
- skuId: `2046514150500524037`
- modes: FREE_CREATION
- billing hint: `unknown`

### happyhorse-1.0/text-to-video-nsfw

- group: HappyHorse
- bindingId: `2048699038968385537`
- skuId: `2046514150500524036`
- modes: FREE_CREATION
- billing hint: `unknown`

### google/veo3.1-pro/start-end-to-video/economy

- group: Veo3.1-Pro-Low-Cost
- bindingId: `2044299767141654530`
- skuId: `2018599147311271938`
- modes: FIRST_LAST_FRAME
- billing hint: `paid_seen`
- params seen in fee-preview only: aspectRatio, creationMode, duration, firstFrameUrl, prompt, resolution
- fee sample: free=False points=88400 cash=0.13 quota=0 params={'aspectRatio': '9:16', 'creationMode': 'firstLastFrame', 'duration': '8', 'firstFrameUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'resolution': '720p'}

### google/veo3.1-pro/text-to-video/economy

- group: Veo3.1-Pro-Low-Cost
- bindingId: `2044303906835816450`
- skuId: `2005884653783007234`
- modes: FREE_CREATION
- billing hint: `unknown`

### google/veo3.1-fast/reference-to-video/stable

- group: Veo3.1-Fast-Official
- bindingId: `2056305155177201666`
- skuId: `2054086928526929953`
- modes: SUBJECT_REFERENCE
- billing hint: `paid_seen`
- params seen in fee-preview only: aspectRatio, creationMode, generateAudio, imageUrls, prompt, resolution
- fee sample: free=False points=489600 cash=0.72 quota=0 params={'aspectRatio': '16:9', 'creationMode': 'subjectRef', 'generateAudio': 'false', 'imageUrls': ['https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png'], 'resolution': '1080p'}

### google/veo3.1-fast/image-to-video/stable

- group: Veo3.1-Fast-Official
- bindingId: `2044300909317742593`
- skuId: `2022225870330286082`
- modes: FIRST_LAST_FRAME
- billing hint: `paid_seen`
- params seen in fee-preview only: aspectRatio, creationMode, duration, generateAudio, imageUrl, prompt, resolution
- fee sample: free=False points=462400 cash=0.68 quota=0 params={'aspectRatio': '9:16', 'creationMode': 'firstLastFrame', 'duration': '8', 'generateAudio': False, 'imageUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'resolution': '720p'}

### google/veo3.1-fast/text-to-video/stable

- group: Veo3.1-Fast-Official
- bindingId: `2044304543057207297`
- skuId: `2022220635650150401`
- modes: FREE_CREATION
- billing hint: `unknown`

### google/veo3.1-fast/start-end-to-video/economy

- group: Veo3.1-Fast-Low-Cost
- bindingId: `2044261714113093634`
- skuId: `2016052223404204034`
- modes: FIRST_LAST_FRAME
- billing hint: `unknown`

### google/veo3.1-fast/text-to-video/economy

- group: Veo3.1-Fast-Low-Cost
- bindingId: `2044302636406300674`
- skuId: `2005884261993070594`
- modes: FREE_CREATION
- billing hint: `unknown`

### 海螺-2.3-图生视频-pro

- group: Hailuo2.3 Pro
- bindingId: `2049089576389341185`
- skuId: `2011758831593648131`
- modes: FREE_CREATION
- billing hint: `unknown`

### 海螺-2.3-文生视频-pro

- group: Hailuo2.3 Pro
- bindingId: `2049100357059301378`
- skuId: `2011999330699112450`
- modes: FREE_CREATION
- billing hint: `unknown`

### Vidu-首尾帧生视频-q3-pro

- group: ViduQ3-Pro
- bindingId: `2044418820103507969`
- skuId: `2019687963136692226`
- modes: FIRST_LAST_FRAME
- billing hint: `paid_seen`
- params seen in fee-preview only: audio, creationMode, duration, firstImageUrl, movementAmplitude, prompt, resolution
- fee sample: free=False points=598400 cash=0.88 quota=0 params={'audio': False, 'creationMode': 'firstLastFrame', 'duration': '8', 'firstImageUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'movementAmplitude': 'auto', 'resolution': '720p'}
- fee sample: free=False points=598400 cash=0.88 quota=0 params={'audio': False, 'creationMode': 'firstLastFrame', 'duration': '8', 'firstImageUrl': 'https://rh-hk-qc-files.xiaoyaoyou.com/77188825b8b3fad12a43824a101850a8/uploads/56d9212772984acad0440937539737a8.png', 'movementAmplitude': 'large', 'resolution': '720p'}

### Vidu-文生视频-q3-pro

- group: ViduQ3-Pro
- bindingId: `2044418820103507978`
- skuId: `2017148354741735426`
- modes: FREE_CREATION
- billing hint: `unknown`

### Vidu-首尾帧生视频-q3-pro-fast

- group: ViduQ3-Pro-Fast
- bindingId: `2043982835939053570`
- skuId: `2034917373414539268`
- modes: FIRST_LAST_FRAME
- billing hint: `unknown`

### Vidu-文生视频-q3-pro-fast

- group: ViduQ3-Pro-Fast
- bindingId: `2044418820103507977`
- skuId: `2034917373414539266`
- modes: FREE_CREATION
- billing hint: `unknown`

## fee-preview coverage in this HAR

- bindings with free quote seen: 2
- bindings with paid quote seen: 11
