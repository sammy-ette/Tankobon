package release

import "testing"

// TestParseFileSeries covers one case per mangaSeriesRegex comment, plus known edge cases.
// Filenames are given without extension; ".cbz" is appended before calling ParseFile.
// By the way, these names are fake (obviously?) thanks claude ^_^
func TestParseFileSeries(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		// Thai Volume: เล่ม n
		{"thai_vol", "Fake Title เล่ม 5", "Fake Title"},
		// Russian Volume: Том n
		{"russian_vol_tom_n", "Fake Title Том 5", "Fake Title"},
		// Russian Volume: n Том
		{"russian_vol_n_tom", "Fake Title 5 Том", "Fake Title"},
		// Russian Chapter: n Главa
		{"russian_ch_n_glava", "Fake Title 1 5 Глава", "Fake Title"},
		// Russian Chapter: Главы n
		{"russian_ch_glava_n", "Fake Title Глава 5", "Fake Title"},
		// Crystal Shore Dreaming - SP02
		{"sp_marker", "Crystal Shore Dreaming - SP02", "Crystal Shore Dreaming"},
		// Lost Shadow World - Vol 005 - Chapter 026
		{"vol_and_chapter", "Lost Shadow World - Vol 005 - Chapter 026", "Lost Shadow World"},
		// Hanamura Kaisenchi - Vol. 30 Ch. 187.5
		{"nagasarete_vol_ch", "Hanamura Kaisenchi - Vol. 30 Ch. 187.5", "Hanamura Kaisenchi"},
		// Haruki_Mae_no_Shogun_v04_ch34 (underscores become spaces)
		{"v_volume_underscore", "Haruki_Mae_no_Shogun_v04_ch34", "Haruki Mae no Shogun"},
		// Yorugata no Valdris - c001-008
		{"dash_c_chapter", "Yorugata no Valdris - c001-008", "Yorugata no Valdris"},
		// Red Arrow - v4 c17
		{"dash_v_volume", "Red Arrow - v4 c17", "Red Arrow"},
		// Yamamoto Kenji - Ghost Dance Musume, Chapter 19
		{"comma_chapter", "Yamamoto Kenji - Ghost Dance Musume, Chapter 19", "Yamamoto Kenji - Ghost Dance Musume"},
		// Please Stay Put, Takeda-Kun! - Chapter 038
		{"double_sep_chapter", "Please Stay Put, Takeda-Kun! - Chapter 038", "Please Stay Put, Takeda-Kun!"},
		// Lost Sea - Digital Colored Comics Vol. 20
		{"vol_dot_number", "Lost Sea - Digital Colored Comics Vol. 20", "Lost Sea - Digital Colored Comics"},
		// Reikon Rettou Chapter 001 Volume 1
		{"chapter_then_volume", "Reikon Rettou Chapter 001 Volume 1", "Reikon Rettou"},
		// Reikon Rettou T3 (Tome short form)
		{"t_volume_short", "Reikon Rettou T3", "Reikon Rettou"},
		// Reikon Rettou Volume 1
		{"volume_word", "Reikon Rettou Volume 1", "Reikon Rettou"},
		// Heirs of Caldonia c000
		{"c_zero_chapter", "Heirs of Caldonia c000", "Heirs of Caldonia"},
		// Starlight Parade [Volume 11]
		{"bracket_volume", "Starlight Parade [Volume 11]", "Starlight Parade"},
		// Darling in the Nexus - Volume 01
		{"dash_volume_word", "Darling in the Nexus - Volume 01", "Darling in the Nexus"},
		// Zara The Storm Bringer - Chapter 027
		{"dash_chapter_word", "Zara The Storm Bringer - Chapter 027", "Zara The Storm Bringer"},
		// Silver Shore - SP02 (via chapter/sp regex)
		{"sp_via_ch_regex", "Silver Shore - SP02", "Silver Shore"},
		// Worlds Greatest Warrior Hiroshi_v11_c90-98
		{"v_and_c_chapters", "Worlds_Greatest_Warrior_Hiroshi_v11_c90-98", "Worlds Greatest Warrior Hiroshi"},
		// Sakura ga BLAST! 018 (2019) (Digital)
		{"chapter_year_meta", "Sakura ga BLAST! 018 (2019) (Digital)", "Sakura ga BLAST!"},
		// Dragon Chaser - Brand New Day 006.5 (2019)
		{"decimal_chapter_year", "Dragon Chaser - Brand New Day 006.5 (2019)", "Dragon Chaser - Brand New Day"},
		// Chapter range with single year (e.g. 001-050 (2019))
		{"chapter_range_single_year", "Test Manga 001-050 (2019) (Digital)", "Test Manga"},
		// Chapter range with year range (the original bug)
		{"chapter_range_year_range", "Even the Knight Commander Wants to be a Hero 001-050 (2023-2026) (Digital) (Oak) [Complete]", "Even the Knight Commander Wants to be a Hero"},
		// Celestine - Episode 429
		{"episode_number", "Celestine - Episode 429", "Celestine"},
		// Sakura ga HUNT! ZERO (2016-2019)
		{"year_range_paren", "Sakura ga HUNT! ZERO (2016-2019)", "Sakura ga HUNT! ZERO"},
		// Definitely Not Kawaii (Ch 59-67)
		{"paren_ch_range", "Definitely Not Kawaii (Ch 59-67)", "Definitely Not Kawaii"},
		// Darkwood Alchemist chapters 101-108
		{"chapters_word_range", "Darkwood Alchemist chapters 101-108", "Darkwood Alchemist"},
		// It's Fighting Time! 001 (Digital)
		{"number_then_paren", "It's Fighting Time! 001 (Digital)", "It's Fighting Time!"},
		// Nigatsu_ni_Sawatta_Toki_v01_ch01
		{"v_ch_underscore", "Nigatsu_ni_Sawatta_Toki_v01_ch01", "Nigatsu ni Sawatta Toki"},
		// Haruki_Otoko_no_Ko_v1.1 (version with decimal)
		{"v_decimal_version", "Haruki_Otoko_no_Ko_v1.1", "Haruki Otoko no Ko"},
		// Red_Arrow_volume_1 (regex requires _ but _ is converted to space before matching;
		// still expects correct series via another regex path)
		{"underscore_volume", "Red_Arrow_volume_1", "Red Arrow"},
		// Tanoshiikuyo MS vol01 chp02
		{"vol_chp_combo", "Tanoshiikuyo_MS_vol01_chp02", "Tanoshiikuyo MS"},
		// Kenjutsushi to Deshi Chp. 1 (no double separator to isolate Chp regex)
		{"chp_dot", "Kenjutsushi to Deshi Chp. 1", "Kenjutsushi to Deshi"},
		// Ghost Festival - Chapter 01
		{"dash_chapter_num", "Ghost Festival - Chapter 01", "Ghost Festival"},
		// Darkwood Alchemist chapters (catch-all form)
		{"chapters_word_catch", "Darkwood Alchemist chapters 101", "Darkwood Alchemist"},
		// Yamikaze - episode 3
		{"umineko_episode", "Yamikaze - episode 3", "Yamikaze"},
		// Kagebira ch01-05
		{"ch_range_no_space", "Kagebira ch01-05", "Kagebira"},
		// Ryuu - Ch.252-005
		{"ch_dot_range", "Ryuu - Ch.252-005", "Ryuu"},
		// Korean: 권
		{"korean_volume", "소설 001권", "소설"},
		// Brighter than White Omake-1 (leading group stripped, _ -> space)
		{"number_suffix_omake", "[FNS]_Brighter_than_White_Omake-1", "Brighter than White Omake"},
		// Asagiri Bousou Biyori - 01
		{"dash_bare_number", "Asagiri Bousou Biyori - 01", "Asagiri Bousou Biyori"},
		// Brighter than White c1
		{"c_bare_number", "[FNS]_Brighter_than_White_c1", "Brighter than White"},
		// Japanese Volume: 第n巻
		{"japanese_volume", "Fake Title 第5巻", "Fake Title"},

		// ---- known problematic cases ----

		// complete collection descriptor after dash should be stripped
		{"complete_collection_suffix", "The Boy I Like is So Charming! - The Complete Manga Collection (2022) (Digital) (1r0n)", "The Boy I Like is So Charming!"},
		// chapter range "001-050 as v01-05" — should strip numeric range and v-range, leaving just the title
		{"chapter_range_as_volume_range", "Even the Knight Commander Wants to be a Hero 001-050 as v01-05 (Digital-Compilation) (Oak) [Complete]", "Even the Knight Commander Wants to be a Hero"},

		// ---- real-world torrent name failures ----

		// number in title treated as chapter
		{"number_in_title", "My Robot is 3 Meters Wide", "My Robot is 3 Meters Wide"},
		// [YYYY-YYYY] bracket range partially consumed, leaving dangling [YYYY
		{"sq_bracket_year_range", "The Boy Upstairs (Digital) (LINE Webtoon) [2019-2022]", "The Boy Upstairs"},
		// same issue, non-latin title
		{"sq_bracket_year_range_korean", "어떤 나라의 이야기 (아무개) [2022-2024]", "어떤 나라의 이야기"},
		// group name contains comma — stripping should still work
		{"group_with_comma", "Wonder Dog Woo-chan (2021-2023) (Digital) (danke-Empire, t3dio)", "Wonder Dog Woo-chan"},
		// complex paren with number inside leaves partial paren in result
		{"complex_paren_with_number", "A Magical Proposal (Digital 2018 000-102+Side Story Tapas)", "A Magical Proposal"},
		// nested bracket: [Translated (Inner Group)] must be fully stripped
		{"nested_bracket_translated", "[Fake Pub! TL] Even the Knight Commander Wants to be a Hero  [Translated  (Fake Pub! TL)][Digital]", "Even the Knight Commander Wants to be a Hero"},
		// leading bracket is the series title, not a scanlation group
		{"bracket_series_title_vol", "[Yume No Hana] - Volume 01", "Yume No Hana"},
		{"bracket_series_title_ch", "[Yume No Hana] 001", "Yume No Hana"},
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
