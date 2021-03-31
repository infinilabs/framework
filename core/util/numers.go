/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package util

func MinInt64(x int64, y int64)int64{
	if x==0{
		return y
	}

	if y==0{
		return x
	}

	if x>y{
		return y
	}else{
		return x
	}
}

func MaxInt64(x int64, y int64)int64{
	if x==0{
		return y
	}

	if y==0{
		return x
	}

	if x>y{
		return x
	}else{
		return y
	}
}
