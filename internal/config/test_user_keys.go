package config

// These keys match the standard User schema's locale and language picklists.
// Keep the accepted combinations explicit: a valid language and country do not
// necessarily form a supported Salesforce locale.
const testUserLocaleKeys = `
af_ZA am_ET ar_AE ar_BH ar_DZ ar_EG ar_IQ ar_JO ar_KW ar_LB ar_LY ar_MA
ar_OM ar_QA ar_SA ar_SD ar_TN ar_YE az_AZ be_BY bg_BG bn_BD bn_IN bs_BA
ca_ES cs_CZ cy_GB da_DK de_AT de_BE de_CH de_DE de_LI de_LU dz_BT el_CY
el_GR en_AD en_AE en_AG en_AL en_AT en_AU en_BA en_BB en_BE en_BG en_BM
en_BS en_BW en_BZ en_CA en_CH en_CM en_CY en_CZ en_DE en_DK en_DM en_EE
en_ER en_ES en_FI en_FJ en_FK en_FR en_GB en_GD en_GH en_GI en_GM en_GR
en_GY en_HK en_HR en_HU en_ID en_IE en_IL en_IN en_IS en_IT en_JM en_JP
en_KE en_KN en_KR en_KY en_LC en_LI en_LR en_LT en_LU en_LV en_MC en_ME
en_MG en_MK en_MT en_MU en_MW en_MY en_NA en_NG en_NL en_NO en_NZ en_PG
en_PH en_PK en_PL en_PT en_RO en_RS en_RW en_SB en_SC en_SE en_SG en_SH
en_SI en_SK en_SL en_SX en_SZ en_TH en_TO en_TR en_TT en_TW en_TZ en_UG
en_US en_VC en_VU en_WS en_ZA es_AD es_AR es_BO es_CL es_CO es_CR es_DO
es_EC es_ES es_GT es_HN es_MX es_NI es_PA es_PE es_PR es_PY es_SV es_US
es_UY es_VE et_EE eu_ES fi_FI fr_BE fr_CA fr_CH fr_FR fr_GN fr_HT fr_KM
fr_LU fr_MA fr_MC fr_MR fr_WF ga_IE gu_IN haw_US hi_IN hmn_US hr_HR ht_HT
ht_US hu_HU hy_AM in_ID is_IS it_CH it_IT iw_IL ja_JP ji_US ka_GE kk_KZ
kl_GL km_KH kn_IN ko_KR ky_KG lb_LU lo_LA lt_LT lu_CD lv_LV mi_NZ mk_MK
ml_IN mr_IN ms_BN ms_MY mt_MT my_MM ne_NP nl_AW nl_BE nl_NL nl_SR no_NO
pa_IN pl_PL ps_AF pt_AO pt_BR pt_CV pt_MZ pt_PT pt_ST rm_CH rn_BI ro_MD
ro_RO ru_AM ru_BY ru_KG ru_KZ ru_LT ru_MD ru_PL ru_RU ru_UA sh_BA sh_CS
sh_ME sk_SK sl_SI sm_US sm_WS so_DJ so_SO sq_AL sr_BA sr_CS sr_RS sv_FI
sv_SE sw_KE ta_IN ta_LK te_IN tg_TJ th_TH ti_ET tl_PH tr_TR uk_UA ur_PK
uz_LATN_UZ vi_VN xh_ZA yo_BJ zh_CN zh_CN_PINYIN zh_CN_STROKE zh_HK zh_HK_STROKE zh_MO zh_MY zh_SG
zh_TW zh_TW_STROKE zu_ZA
`

const testUserLanguageKeys = `
da de en_US es es_MX fi fr it ja ko nl_NL no
pt_BR ru sv th zh_CN zh_TW
`
