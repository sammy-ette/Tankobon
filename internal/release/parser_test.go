package release

import "testing"

// TestParseFileSeries covers one case per mangaSeriesRegex comment, plus known edge cases.
// Filenames are given without extension; ".cbz" is appended before calling ParseFile.
func TestParseFileSeries(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		// Thai Volume: เล่ม n
		{"thai_vol", "Manga Title เล่ม 5", "Manga Title"},
		// Russian Volume: Том n
		{"russian_vol_tom_n", "Manga Title Том 5", "Manga Title"},
		// Russian Volume: n Том
		{"russian_vol_n_tom", "Manga Title 5 Том", "Manga Title"},
		// Russian Chapter: n Главa
		{"russian_ch_n_glava", "Manga Title 1 5 Глава", "Manga Title"},
		// Russian Chapter: Главы n
		{"russian_ch_glava_n", "Manga Title Глава 5", "Manga Title"},
		// Grand Blue Dreaming - SP02
		{"sp_marker", "Grand Blue Dreaming - SP02", "Grand Blue Dreaming"},
		// Mad Chimera World - Vol 005 - Chapter 026
		{"vol_and_chapter", "Mad Chimera World - Vol 005 - Chapter 026", "Mad Chimera World"},
		// Nagasarete Airantou - Vol. 30 Ch. 187.5
		{"nagasarete_vol_ch", "Nagasarete Airantou - Vol. 30 Ch. 187.5", "Nagasarete Airantou"},
		// Ichiban_Ushiro_no_Daimaou_v04_ch34 (underscores become spaces)
		{"v_volume_underscore", "Ichiban_Ushiro_no_Daimaou_v04_ch34", "Ichiban Ushiro no Daimaou"},
		// Gokukoku no Brynhildr - c001-008
		{"dash_c_chapter", "Gokukoku no Brynhildr - c001-008", "Gokukoku no Brynhildr"},
		// Black Bullet - v4 c17
		{"dash_v_volume", "Black Bullet - v4 c17", "Black Bullet"},
		// Kedouin Makoto - Corpse Party Musume, Chapter 19
		{"comma_chapter", "Kedouin Makoto - Corpse Party Musume, Chapter 19", "Kedouin Makoto - Corpse Party Musume"},
		// Please Go Home, Akutsu-San! - Chapter 038
		{"double_sep_chapter", "Please Go Home, Akutsu-San! - Chapter 038", "Please Go Home, Akutsu-San!"},
		// One Piece - Digital Colored Comics Vol. 20
		{"vol_dot_number", "One Piece - Digital Colored Comics Vol. 20", "One Piece - Digital Colored Comics"},
		// Kyochuu Rettou Chapter 001 Volume 1
		{"chapter_then_volume", "Kyochuu Rettou Chapter 001 Volume 1", "Kyochuu Rettou"},
		// Kyochuu Rettou T3 (Tome short form)
		{"t_volume_short", "Kyochuu Rettou T3", "Kyochuu Rettou"},
		// Kyochuu Rettou Volume 1
		{"volume_word", "Kyochuu Rettou Volume 1", "Kyochuu Rettou"},
		// Knights of Sidonia c000
		{"c_zero_chapter", "Knights of Sidonia c000", "Knights of Sidonia"},
		// Tonikaku Cawaii [Volume 11]
		{"bracket_volume", "Tonikaku Cawaii [Volume 11]", "Tonikaku Cawaii"},
		// Darling in the FranXX - Volume 01
		{"dash_volume_word", "Darling in the FranXX - Volume 01", "Darling in the FranXX"},
		// Momo The Blood Taker - Chapter 027
		{"dash_chapter_word", "Momo The Blood Taker - Chapter 027", "Momo The Blood Taker"},
		// Grand Blue - SP02 (via chapter/sp regex)
		{"sp_via_ch_regex", "Grand Blue - SP02", "Grand Blue"},
		// Historys Strongest Disciple Kenichi_v11_c90-98
		{"v_and_c_chapters", "Historys Strongest Disciple Kenichi_v11_c90-98", "Historys Strongest Disciple Kenichi"},
		// Hinowa ga CRUSH! 018 (2019) (Digital)
		{"chapter_year_meta", "Hinowa ga CRUSH! 018 (2019) (Digital)", "Hinowa ga CRUSH!"},
		// Goblin Slayer - Brand New Day 006.5 (2019)
		{"decimal_chapter_year", "Goblin Slayer - Brand New Day 006.5 (2019)", "Goblin Slayer - Brand New Day"},
		// Chapter range with single year (e.g. 001-050 (2019))
		{"chapter_range_single_year", "Some Manga 001-050 (2019) (Digital)", "Some Manga"},
		// Chapter range with year range (the original bug)
		{"chapter_range_year_range", "Even the Elf Captain Wants to be a Maiden 001-050 (2023-2026) (Digital) (Oak) [Complete]", "Even the Elf Captain Wants to be a Maiden"},
		// Noblesse - Episode 429
		{"episode_number", "Noblesse - Episode 429", "Noblesse"},
		// Akame ga KILL! ZERO (2016-2019)
		{"year_range_paren", "Akame ga KILL! ZERO (2016-2019)", "Akame ga KILL! ZERO"},
		// Tonikaku Kawaii (Ch 59-67)
		{"paren_ch_range", "Tonikaku Kawaii (Ch 59-67)", "Tonikaku Kawaii"},
		// Fullmetal Alchemist chapters 101-108
		{"chapters_word_range", "Fullmetal Alchemist chapters 101-108", "Fullmetal Alchemist"},
		// It's Witching Time! 001 (Digital)
		{"number_then_paren", "It's Witching Time! 001 (Digital)", "It's Witching Time!"},
		// Ichinensei_ni_Nacchattara_v01_ch01
		{"v_ch_underscore", "Ichinensei_ni_Nacchattara_v01_ch01", "Ichinensei ni Nacchattara"},
		// Kasumi_Otoko_no_Ko_v1.1 (version with decimal)
		{"v_decimal_version", "Kasumi_Otoko_no_Ko_v1.1", "Kasumi Otoko no Ko"},
		// Black_Bullet_volume_1 (regex requires _ but _ is converted to space before matching;
		// still expects correct series via another regex path)
		{"underscore_volume", "Black_Bullet_volume_1", "Black Bullet"},
		// Amaenaideyo MS vol01 chp02
		{"vol_chp_combo", "Amaenaideyo_MS_vol01_chp02", "Amaenaideyo MS"},
		// Mahoutsukai to Deshi Chp. 1 (no double separator to isolate Chp regex)
		{"chp_dot", "Mahoutsukai to Deshi Chp. 1", "Mahoutsukai to Deshi"},
		// Corpse Party - Chapter 01
		{"dash_chapter_num", "Corpse Party - Chapter 01", "Corpse Party"},
		// Fullmetal Alchemist chapters (catch-all form)
		{"chapters_word_catch", "Fullmetal Alchemist chapters 101", "Fullmetal Alchemist"},
		// Umineko - episode 3
		{"umineko_episode", "Umineko - episode 3", "Umineko"},
		// Baketeriya ch01-05
		{"ch_range_no_space", "Baketeriya ch01-05", "Baketeriya"},
		// Magi - Ch.252-005
		{"ch_dot_range", "Magi - Ch.252-005", "Magi"},
		// Korean: 권
		{"korean_volume", "만화 001권", "만화"},
		// Darker than Black Omake-1 (leading group stripped, _ -> space)
		{"number_suffix_omake", "[BAA]_Darker_than_Black_Omake-1", "Darker than Black Omake"},
		// Akiiro Bousou Biyori - 01
		{"dash_bare_number", "Akiiro Bousou Biyori - 01", "Akiiro Bousou Biyori"},
		// Darker than Black c1
		{"c_bare_number", "[BAA]_Darker_than_Black_c1", "Darker than Black"},
		// Japanese Volume: 第n巻
		{"japanese_volume", "Manga Title 第5巻", "Manga Title"},

		// ---- known problematic cases ----

		// complete collection descriptor after dash should be stripped
		{"complete_collection_suffix", "The Girl I Want is So Handsome! - The Complete Manga Collection (2022) (Digital) (1r0n)", "The Girl I Want is So Handsome!"},
		// chapter range "001-050 as v01-05" — should strip numeric range and v-range, leaving just the title
		{"chapter_range_as_volume_range", "Even the Elf Captain Wants to be a Maiden 001-050 as v01-05 (Digital-Compilation) (Oak) [Complete]", "Even the Elf Captain Wants to be a Maiden"},

		// ---- real-world torrent name failures ----

		// number in title treated as chapter
		{"number_in_title", "My Girlfriend is 8 Meters Tall", "My Girlfriend is 8 Meters Tall"},
		// [YYYY-YYYY] bracket range partially consumed, leaving dangling [YYYY
		{"sq_bracket_year_range", "The Girl Downstairs (Digital) (LINE Webtoon) [2019-2022]", "The Girl Downstairs"},
		// same issue, non-latin title
		{"sq_bracket_year_range_korean", "그렇고 그런 바람에 (아니영) [2022-2024]", "그렇고 그런 바람에"},
		// group name contains comma — stripping should still work
		{"group_with_comma", "Wonder Cat Kyuu-chan (2021-2023) (Digital) (danke-Empire, t3dio)", "Wonder Cat Kyuu-chan"},
		// complex paren with number inside leaves partial paren in result
		{"complex_paren_with_number", "A Business Proposal (Digital 2018 000-102+Side Story Tapas)", "A Business Proposal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFile(tt.filename + ".cbz")
			if got.Series != tt.want {
				t.Errorf("Series = %q, want %q", got.Series, tt.want)
			}
		})
	}
}
