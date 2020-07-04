// SPDX-License-Identifier: Unlicense OR MIT

package org.gioui;

import android.app.Activity;
import android.content.res.Configuration;
import android.os.Build;
import android.os.Bundle;
import android.view.View;
import android.view.Window;
import android.view.WindowManager;

public final class GioActivity extends Activity {
	private GioView view;

	@Override public void onCreate(Bundle state) {
		super.onCreate(state);

		Window w = getWindow();

		this.view = new GioView(this);
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
			this.view.setSystemUiVisibility(View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION | View.SYSTEM_UI_FLAG_LAYOUT_STABLE);
		}
		this.view.setLayoutParams(new WindowManager.LayoutParams(WindowManager.LayoutParams.MATCH_PARENT, WindowManager.LayoutParams.MATCH_PARENT));
		setContentView(view);
	}

	@Override public void onDestroy() {
		view.destroy();
		super.onDestroy();
	}

	@Override public void onStart() {
		super.onStart();
		view.start();
	}

	@Override public void onStop() {
		view.stop();
		super.onStop();
	}

	@Override public void onConfigurationChanged(Configuration c) {
		super.onConfigurationChanged(c);
		view.configurationChanged();
	}

	@Override public void onLowMemory() {
		super.onLowMemory();
		view.lowMemory();
	}

	@Override public void onBackPressed() {
		if (!view.backPressed())
			super.onBackPressed();
	}
}
