package jwt_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/shaj13/libcache"
	_ "github.com/shaj13/libcache/lru"

	"github.com/rubyniu105/framework/lib/guardian/auth/claims"
	"github.com/rubyniu105/framework/lib/guardian/auth/strategies/oauth2/jwt"
	"github.com/rubyniu105/framework/lib/guardian/auth/strategies/token"
)

// nolint:lll
const (
	idToken     = "eyJhbGciOiJSUzI1NiIsImtpZCI6ImZkYjQwZTJmOTM1M2M1OGFkZDY0OGI2MzYzNGU1YmJmNjNlNGY1MDIiLCJ0eXAiOiJKV1QifQ.eyJhZGRyZXNzIjp7fSwiZW1haWwiOiJleGFtcGxlQGV4YW1wbGUuY29tIn0.C7OH90Uue3Xj5Y1jxO7PRbwglmqdNfY080zrL8erOAuZ770FQVheijYgZgKa_pMlB5gYwqBojRW3tzGIvyly-1Y8jqEZ4TVhtZFos-ccz07r83VmuU0QFzJlvFKglonPviEiKlzT5mn1XmICIdGC9eypwRBY1Z-6qSybJ23YVaxLLm10iMu8aiX_UKP34pidtSHX52erQCGSypEeUmHPGrdk6rqfS3vWS5JfnjUg5tEtNplfK76lWHnSR707DIcK45yLpzQpkI4nDN4u1djs5vnSpCTyzIMW66guiPEWjDtnlob5jxFkhRwnSZcx8H6Ajc5K-DzjhIv5VpRFpSRNUYKLg__GYA4rMkghmGKzSe3KRBpazJND2fJZxdm0TqNiTxFoUNZm0DD6cTcaAdJ6fd9Lv12asI02bnJjohrCn3UVWrcRTMJ2LbDrBK5301gdArNfCva4vYs5z1qpaljbG8wdaJYZw9ptJNnRm1CPfYfZw7z0HAUnq5282zJo1VDrE_NnFfWmwxUsxYN98h-bHj6RKC-j4tNKAlHokF3pBfZisJSjHOjS6pFcwgv7Nmc4Tsk9d3rH_kWalOyJJ23YejF8l2tjXTkPw5MGl4OsaM5_mSNy5FwBTqEebdjuLfELTtXXYG7BSGkl5I60TrOnZaQ_lYLxyOGq-61YnYjkUJU"
	accessToken = "eyJhbGciOiJSUzI1NiIsImtpZCI6ImZkYjQwZTJmOTM1M2M1OGFkZDY0OGI2MzYzNGU1YmJmNjNlNGY1MDIiLCJ0eXAiOiJKV1QifQ.eyJleHAiOjQ3Njc5NTQwNjMsInNjb3BlIjpbImRvbHBoaW4iXSwic3ViIjoiZXhhbXBsZSJ9.aYHFFLBb1wj9hCNxfZ5s8kEC1ZyH62LOyK2GK6rn5ZlkicEzgg5IaNiLj6oumvOYP520GVJA0BEvNV8yZyYNWjzO66RW5ZPT6ymcDvR2Bv8wlMnz4X03tqdygOk6P6sb3nmD0RjNlUzC3bIsU46ftleQdmd7Db9Wimn9CVrAFoNwsSFvHG2VDp_b14Z7reUsKUuJtTMd66S1XPyF5zDHoHpru1T68H-wEq5R082htAwV4l9ACJIxMsOeenXVXq1QXCn9khIPb8aU1IUjzDvn6hjw1-qyfcGPfyfj83CErfnqXbdaRHSPhNjWwC6oZPr5fLRoiRF-9YqxfY55l_znPDf34KxXpEu6e8g7nLNJQKGpZTnX952mWqMItYyyQDQV80EL3sC9jyTXve2D9mqTAYVb3aRqLaUJp3kxR4M4WwbYBTf02GIr2g1lntkzc95W7R925EXyDwz06Yi-5PQFFn-EJgTMzYLdutW-G7a44FxZVurQlFeMnDZmi1Z1DWWKPy6NZzTxyPjFSwgfcxLp4YOP9HKM4dYWzuMvvYk4DWPk2oDkU8dy40WzCXF9rmnOYdQoseVMrK5Ku6fC31QzfjcXdvEYN0sys5gPyKpU_LSS0K1FJQhhrwDLFkNDLQpScD87V9aFgAbH-yrBKrgLzcHVFDNTxpJ7sAqsWxH3jJw"
)

func Example() {
	srv := AuthrizationServer()
	opt := jwt.SetHTTPClient(srv.Client())
	strategy := jwt.New(srv.URL, libcache.LRU.New(0), opt)
	r, _ := http.NewRequest("GET", "/protected/resource", nil)
	r.Header.Set("Authorization", "Bearer "+accessToken)
	info, err := strategy.Authenticate(r.Context(), r)
	fmt.Println(info.GetUserName(), err)
	// Output:
	// example <nil>
}

func Example_idtoken() {
	srv := AuthrizationServer()
	opt := jwt.SetClaimResolver(&jwt.IDToken{})
	strategy := jwt.New(srv.URL, libcache.LRU.New(0), opt)
	r, _ := http.NewRequest("GET", "/protected/resource", nil)
	r.Header.Set("Authorization", "Bearer "+idToken)
	info, err := strategy.Authenticate(r.Context(), r)
	fmt.Println(info.(jwt.IDToken).Email, err)
	// Output:
	// example@example.com <nil>
}

func Example_scope() {
	opt := token.SetScopes(token.NewScope("dolphin", "/dolphin", "GET|POST|PUT"))
	srv := AuthrizationServer()
	strategy := jwt.New(srv.URL, libcache.LRU.New(0), opt)
	r, _ := http.NewRequest("DELETE", "/dolphin", nil)
	r.Header.Set("Authorization", "Bearer "+accessToken)
	_, err := strategy.Authenticate(r.Context(), r)
	fmt.Println(err)

	// Output:
	// strategies/token: The access token scopes do not grant access to the requested resource
}

func ExampleSetVerifyOptions() {
	srv := AuthrizationServer()
	client := jwt.SetHTTPClient(srv.Client())
	vopts := jwt.SetVerifyOptions(
		claims.VerifyOptions{
			Issuer: "https://server.example.org",
		})
	strategy := jwt.New(srv.URL, libcache.LRU.New(0), client, vopts)
	r, _ := http.NewRequest("GEt", "/protected/resource", nil)
	r.Header.Set("Authorization", "Bearer "+accessToken)
	info, err := strategy.Authenticate(r.Context(), r)
	fmt.Println(info, err)
	// Output:
	// <nil> strategies/oauth2/jwt: claims: standard claims issuer name does not match the expected issuer
}

func AuthrizationServer() *httptest.Server {
	h := func(w http.ResponseWriter, r *http.Request) {
		// nolint:lll
		const body = `
		{
			"keys": [
			  {
				"use": "sig",
				"kty": "RSA",
				"kid": "fdb40e2f9353c58add648b63634e5bbf63e4f502",
				"alg": "RS256",
				"n": "v7OPSb-6KPCyFegISvszzI_0hKUK67rh0AFh5hkilHfD9uoSxBTG5XrlTwaiILXofYR8SgXWKQRIwJYyfr80V5HDlgg1UDoDUoCNeEBC7xNga5dbXVVLV2w3_P5VI0Z4FVbHPy9WV1qjCs7IgdVoCnMfd2ItDHqNOfP0F4oc4ndbF7Lw0CeoNgFGMeF2fsdTldMp8-BU81Uk6WqMLhUdEHV6ZG94UzFjL1eiFdIPhIMXtCbTUNmn3MOtnAkFAIke7397_dhj7L5C1hGsxfTgoCLnPaInpX5AOQSzdY1StPzDPEYT2ZlRnhBVfK1F_ExZrKWwfGNhdcm9g2cQXTtY4CPk96aKWKxn-szDuWH6fQ8Rd11AARePiBN2jCVo1bvNyjBmSyI_dtH30DP-N0pU3y1RABaJtOMkQSRWX24tRcMDnYYugwjjA1pugoFVx-zL-fTnOY9u-yXsmgf9Isr92jOT26sV0lMh9kWyAgaDtFz9TXtut16FkNBR7uCMtuBZB5oQYXkg0ZfzyVPmo5qKqaH6X8r44z7k4Uf0t14T2Ejxe8kGqxXFqETS1K7vaskCjIL3X3SADB7AfNd4TBTVVb5I3deY44p4wcOLmudnRFSZSdMPu4XHpB3xO-wFU-h_NvMnJFLwzAuP2bMeBQSuKT7xxPUNlWTUvNgLfY44qc8",
				"e": "AQAB",
				"d": "KFZCCkSbiU3MSyu9wvlElwCbdOW9fIigR0JjNSWIzzC8PVJXjIbKqzLG2XAN4VAlkXO1K2Y6__p0zIFOMrlM7DgxrXogrbbnSA7gtbLf4qpzGXCJuwPdjJGq3kMt6vRDBEp0Nmlhg5QAxp9oNVmQQNKkhlxUGlIXMWCRtfpLxaNTuZLfdQ1DKcnu2UQVyOtsPRRnuXc0qNb7o1nWEURED1iI3mVOLkMwGaAY7Pp8ZWeoLzIUOOjzl1JdT33eXZR8u-xZTLqhnAkUyzKA5k52jXuKqL9cFEiSfuzsTgnko0ykUCR2vMy0DcxmEIvtM_9kxx0-G45VzZEbnXCsUtHQC0xx8wIcpmDADYx0W_fv_Fz-C2AgIxh9apD5pg1Mxe9tHhcJxTQfABbiW0G-I-HziBJpBXZIW7V27ngdYDR9a35ZgcgoqxdTCXsJqwplpQsE84bggmN4KeX_8NnCqb8BswpQpvZw690yOEYGV1p7dlfXU1dnw8oBujwwbH0ORS6Gn2eE1UM92Nt4ot6R_FDdj1uWFAA1mIyW7jGmbRG6zhVHlOYhHbcwKQFxMG51OXVWWMCj457wFmp0_56wMpepSrAk8z9U7r5iY4-b9RMCEh7j-BHr0cGPZ00vPmw-V6ABWUycPsKQ3FK38HPEmMH1H6-3J62QfPW8LwVNkm4xcxE",
				"p": "1D6hdpQBwvgxSRpqjgxrrAaTaVFm_7A4T4i4cGpkpogxZBAlVpMeBXwj1OZe2bzLddvcfdFdZAnsidsUyheNekiYkrKQDAgUtqmzD-TraAj83paO94xcoRZzFJsS5h8FFjvR_eolGgWDRQyJrfsgBrr_5aAYlqAQul2-lNav42nMlgL6MaS94pOLDhxVntVk4581KRwhfUsJiIF8pzG6vFiZi5vHmw4_-yiybfRJdI2-QftBx9B-JpXsaEZrmk5_peAyQ8uU3E7DXq4adBoIQYMnAqSstZW3mRGPVZ0boA_FnwfPR79IQkuYl_MIpnhFPKUGH-HGZTbv411jDTBxTQ",
				"q": "5zi-cVCD4qva9IiEBvt3NjiWr4N2xcWi43JuLjPiNaDUiOn8_lmCdXurs4jCfsm3mi4daLdp6lrS24iKWw9yCqIkEiTSpih6drX1IJAFwYD22lCklDMDisexhO9rPGRrX3AlwycarOAzIyCP6DSBahd43P-uhwrPUbfhL1S1oAF_vN92DhBKok0nKAjZdWrI77XN9F54IJW-fWMjsSF1ujvyuvK-fm570xwxx7BkcpIsK5ZEm1or3b3jWQhX9dUFqtAL_pRs4Nw_euQsE6zIN1v0424SPZc7Lt_UeQ--1GQhpuPTv2vmtDXApb-jUuz6KLZGhgKpH5mNz1rnYfU5iw",
				"dp": "ENMCI48p8JWR-pSAe9AaPOGsj72nJ3-FhzB0Rlz4q4bCO4dYHlu9Fnw3rumv_RyNGEOcX9DX0VVEDc1zAW4KhfX5Oi-zYXDGi5A6JHll-7IysUZIAPF8ajyIVMrSHbG5yoBlbfZAiKaFOFT9GPB-ImpyXHZrXI1FpjBGKjA2cxVw5TdJM-Q2NR6y-CRg2R1bSPvWz_Jt6SuojsyM4Af-IG35heqMUQs5ISShuDuUEwwlV7-eAEPTrCVYPw_N-cZdMf3qnhsmKqyHqhqs-CUUIHVQA1Kgaih7DEQrE4NHrFFzvd51nN9Zz_-EEg9u0RtZiawfJynTezR2oZRGhMYhRQ",
				"dq": "dX46F66INeiKDHRKUpn5i83ZlDpDYl_5U4ZUQpoOup2NIj10V3L4feZn64T1ACRUbb49J3b8FSAtwWxyka8ZjhmyJp4bhF9RS31OoEtPAXMc_Pa5iq0Zga3ToO9gGIIWpZqBNddrEKmkkpb7SU1U7aobuoEaGHj_vFCp1rk-yZ25YSpT_PV-V1bJLOjCR44JqPVDQIe4lyZAc8qq2llcT1QjFag_8FMIDNBo40XY5PcuBsAHAMIjRDw3iIha2gpzJMcvMSAO63w_rZzAYQcNfkP1_pNyJWXxpvIKL7I2kAqJpxpiAQU9aBlgWVk2Du9odsOYtoQnmG0YyGMy7G4F3Q",
				"qi": "kejQ_-fEoPXusCxTGZMEE-jALpmouaPZztMXIWii68i9i6TfXvO_btuQTMzsNTCVOgHBRSZ6_kqrsSOAHYq8pPJ0DRnk_HjT3pzSD2RicFYuoE_AkoFVeooVGnCTIk_0n1p2bWItX2ZGNkCPy6CA-73jZ25xTA1CelB8b5jsMBd5B7vY1FYI6beOWhB2Of3dF2eKi0PomMIW4izrnnz1y_p56Y2nJvhVL3zOqomxQKobGS_PzdYN25aqqqcFlZRxEWYrq0biGaREf8kZtF8ZymAIresS2L4PrTyiZzfi2fyyWoLUg57yX6d12oZoZ0vO55g9H0EZGqJ6gy04R4KwyA"
			  },
			  {
				"use": "sig",
				"kty": "RSA",
				"kid": "fed80fec56db99233d4b4f60fbafdbaeb9186c73",
				"alg": "RS256",
				"n": "m5fbFBv2hNCGDVsLc0EGOKIZoGGFlSasrCcvkQNUEDzMXr81aGCobfzYnNFLbAyqdzVHDyMCbhffXnyAuUy4_ji_4H9q8HF1LfnvdCZqI6Q0CG6XVrxJgwY69l8_uzoS1ro_LBhyZmoAYv5WhnBKeja7vBLwKClBKVOrjno659OVD09AK9hFACEWDurzKKN5f-k8ziIlvw4tn4AOd01mQcZoReas4Bs2mrgqxptsb0Ucjc66No3Xdl8p1g5ubf2SONhbMxc8xn7tqUKx-RDP-Sa26VB7CSPxST6BGou3t6WnzByNufHyrbPRmuczk5hrtuSzZDS8GkmDeRp9bLqjn2E6MKa3X0ZPTd51jYrHv75nUhbdXtoro2Px8ctu_gwC7jaoFl9DLzoPyxh4CW7p02MQ_bW6gq9zri_POeKYY-F-uVbZGF2uedn_vZ8CZ_s2EG96FlqFw0vNWqMnrhz1c98NokX4T0laaFbd-4zfIXRWOZBxDbh5jcE0Fb-vWzFvg-9JT_L4SBTKp0M69w-xmvKZkoLCsEKMGyGJjtHzKrrmH10xEPo9M94t6dVXvBm5W7P6H_fzf6SafJ48zwGsTVVPrhZ25xWgVH7rHYhwFSuuXtgtu9DDG-1JENbbw4kobCXbrY4vA-ijph3Nd-dOt3frNWxBupyhlLJCmIT7OUk",
				"e": "AQAB",
				"d": "aECvwiGaZBN0Pq6qVWdUS84RbazqXK21NQRskrWwNdEG_tUPbAiX0lqAqVJzPsqdzZIdMr86eZn1SNITThViPrS3nCzD8qeS5GN7VlAG_iqf0qaHMM6oUupxx3K6uTCIPug8O8eFn6mW6L2SLDJBNPJHiBUIZWB_ELnHUYgEwCC81606SiZ21UdWCFjU5H3kgxg8bcHjmMhfOWgMSVPLGHdglrWhT-fsBm8v-jNZzJR6NWo2yybvH5lT5uF0jK5Cs2QEd48yYa3agHb32PKy5zZRiLMsPUuf-Huw9aB4UMzmSZU4QUckW88Iusn_fP277qf-qz3Ka7KmLRbaw2erCawJEsOY97-R9cZ9QTmKvVuwvj3UkNuJSzzPmf9R80lkwiMvEqONLVDn7wx476ViW0tgfc921mS_1t4s0eZ2DilSTLj2RxeauiCufBTp2fLTVU9w5iXJA4uFZYe3nPBGb32H0qtyHy9dP23KVbhAHyxGuc6Ji7frVAR7tRXRHGY8TN0C4cASJu4lUSp3gAhoriC9Cfyno-VeWWMCfmB8g7cYoEy5rzOk0H2CYYBN5hurjaXd2xEa3UOxAO28uw7OlvVzFUHzLl6t1lpi94r-h39AR3ebaHbVa1M0UGqXoVmbHdFflm0MAfKL8B_bKKJYMFLYQzcUc0oO3K1IT3pf_qE",
				"p": "zMcc4f6oaYpEmOzOYj1_me1o2EsUs0_9G13VyxKDPrEbGjK3DbguRhDmfZ6u7xE201kmVkTfPMQ4CK6fe3CAqnIfnRm9ziKIU22jhNgfSo59xGBmR5BRJJaKLbZfNz9ubM4XJbeE36BXR6XV1HGctxo0TmADw2jdEwwKwSp5cJRQqYZ3y4WHwTqcR1zV0oiMP0FWwv983rn0WhAjw7d6Bp2AgO8GWYsejIL5oGWULHh8DU0QofS6u2mmKBai2wEf_wkSn8j1S3Rs52TU5eKz8G_B_xH-z_O-Mf-v8K3d9fTn0wCi8qHKs6MMfOQodE-JOIIM4qHk0vxrlNHDaJTqxw",
				"q": "woM4oehELLLQ4f1MbyDCmaTOtx2_QtQl2ftH_O8A3m-PQIzAQ-EAH_IFXcCcaONhk50cQjCgKZDwT656S99WXqSPlgE1E1ZMf3phuDByZ4p4NUE0cURlxHl0dRYOQbWmR7e69VFJEzuW5Rgh-Wh8IczAb20vErulEX-fEDSmerUgAMiucR1ckH6mkmnkPGj2BTlLyG2FrZcdNdh1t-9FgeN28_xLMPjzNmn58EwVro-vdwmV8Pzd0wwnG3NO7ERx60ZPyUYr5AgOiK-MOfPsgfNAgQmkXWZ2ii2be9e7aAjMlmdU-Cl4FaB2TQkpaBQbG66dXxd_DF9lfO2ERTYrbw",
				"dp": "NqpErJPFs37ktwooQhN2t8mnvm20lfWZdK_E_dPwU1EGEiVNtozfVXb3gLtWqZ0nzJ203Ty_d0JOTwsGqfYrctTKWa7ge2G-kL7o8vKaz9Vf_4dYZmxBLQo-0tsnaeE2Aje1-CyYfPYZtpevkGnP0xVctztsZcLdmVMSn-RNzN7a9ZZe7ma0CcIyq949ellXTx-LIL0BQZfUgiJi2cFmAtQS1Nh6EndP7WSdbNMRDhoPy6Ex-noRSyx13afFS79uIi_y19LWoJDw7Yh-SOwO6vV6jTPpmOvRbxl5hz9yzFDXff1ignDsYq35DHH_1qTQ1dPpyqo7IpOdyHmCt61hSw",
				"dq": "SP6ZeBkDzIpmXQiDcIiovqPcd1eQePHIKp9kCoVenBrddWnclRyQwWw_m0k26R27dnvVKPm6gR7FMAHYHzT24pl60N4vHsyZ9JTmqwpzRGvwZHvNxFvYnPy_OVlHjF0ww2UtofYZKECKhfqidUhCnSSLasVcjvkgHwr3lEtN1mq2UdT9sbFFFWyR8gwO_KSe_qLbz6FaMySsb5KFyrreKLpF35XkWcJy8w6eHxFOaa2-OTu9qywZyqOa4XBKQ9wDrDk8o9nTisWDPsQyKWVicfnpUQNfTTWwcnZfDQCOcaIrtJ2eg2p8iBEplAtGIKq66Y6DvDXDFc-O9Gzl4FtNvQ",
				"qi": "IUA4aes-L50Kb6fE0yJfsEOaZBefn1nvBnRN4pkMoNIm2XAnv5coYAvx9-V3q6jp2RNxhn__VOdyS02mVtnnEAmF7Dlv5pi4Tn_nGJBjwl0sgTt6IGl-D83bhzyeIlScsLndPuC3qELMt6dSNPdpoy9bvJ3qmrTJqZRQ3popROWA5AUZIaEC7P9AEZ_2htSkU1ETmqVOLhNAHfqLrW9cmcsrjsOF8jnXh5djX2_dCczcy_5WUPjnxUyzEo5Zp3nNHztZ9l2DfkjAEohuRem0odk5A8wx1-6G8pfvfXz3si0stmEhwX7rYwb07aUiWxeAkUcIjhIHHlgn1ThONAMneg"
			  }
			]
		  }
		`
		w.WriteHeader(200)
		w.Write([]byte(body))
	}
	return httptest.NewServer(http.HandlerFunc(h))
}
