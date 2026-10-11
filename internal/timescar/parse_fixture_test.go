package timescar

import "testing"

const fixture = `<html><body>
<p>
<input type="hidden" id="naviPageNo-hidden" name="x" value="1" />
<input type="hidden" id="naviPageCount-hidden" name="x" value="18" />
<input type="hidden" id="naviResultCount-hidden" name="x" value="356" />
</p>
<table border="0">
<tr><th colspan="2">ステーション名</th><th colspan="2">住所</th><th>&nbsp;</th></tr>
<tr width="15" height="15"><td class="noborder"><span id="stationNm">タイムズ川越新宿町第２</span><br /></td>
<td class="right"><a id="goDetail" href="./detail.jsp?scd=IA20&amp;newwindow=true">詳細</a></td>
<td class="noborder"><span id="adr1">埼玉県川越市新宿町2-3</span></td>
<td class="right"><a id="goStationMap" href="./stationMap.jsp?scd=IA20">地図</a></td></tr>
<tr width="15" height="15"><td class="noborder"><span id="stationNm">A&amp;Bステーション</span><br /></td>
<td class="right"><a id="goDetail" href="./detail.jsp?scd=NR62&amp;newwindow=true">詳細</a></td>
<td class="noborder"><span id="adr1">東京都千代田区有楽町2-7</span></td></tr>
</table>
</body></html>`

func TestParseFixture(t *testing.T) {
	res, err := parseResult(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 356 || res.PageCount != 18 {
		t.Fatalf("total/pages = %d/%d", res.Total, res.PageCount)
	}
	if len(res.Stations) != 2 {
		t.Fatalf("got %d stations: %+v", len(res.Stations), res.Stations)
	}
	want := []Station{
		{Code: "IA20", Name: "タイムズ川越新宿町第２", Address: "埼玉県川越市新宿町2-3"},
		{Code: "NR62", Name: "A&Bステーション", Address: "東京都千代田区有楽町2-7"},
	}
	for i, w := range want {
		if res.Stations[i] != w {
			t.Errorf("station %d = %+v, want %+v", i, res.Stations[i], w)
		}
	}
}
