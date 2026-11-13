package com.limoni.lemondrop

import android.app.AlertDialog
import android.os.Build
import android.os.Bundle
import android.text.InputType
import android.view.HapticFeedbackConstants
import android.view.View
import android.widget.EditText
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import com.limoni.lemondrop.mobile.Bridge
import com.limoni.lemondrop.mobile.EbitenView
import com.limoni.lemondrop.mobile.Mobile
import go.Seq

class MainActivity : AppCompatActivity() {
    private lateinit var view: EbitenView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Seq.setContext(applicationContext)
        Mobile.setDataDir(filesDir.absolutePath)

        // Draw edge to edge; the game keeps its controls inside the safe area.
        WindowCompat.setDecorFitsSystemWindows(window, false)
        setContentView(R.layout.activity_main)
        view = findViewById(R.id.ebitenview)
        WindowInsetsControllerCompat(window, view).apply {
            hide(WindowInsetsCompat.Type.statusBars())
            systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        }
        ViewCompat.setOnApplyWindowInsetsListener(view) { _, insets ->
            val i = insets.getInsets(WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout())
            val d = resources.displayMetrics.density.toDouble()
            Mobile.setSafeArea(i.top / d, i.left / d, i.bottom / d, i.right / d)
            insets
        }
        Mobile.setBridge(NativeBridge(this, view))
    }

    override fun onPause() {
        super.onPause()
        Mobile.suspend()
        view.suspendGame()
    }

    override fun onResume() {
        super.onResume()
        view.resumeGame()
    }
}

/** Haptics and the name prompt, called from Go. */
class NativeBridge(private val activity: AppCompatActivity, private val view: View) : Bridge {
    override fun haptic(kind: Long) {
        val c = when (kind.toInt()) {
            0 -> HapticFeedbackConstants.CLOCK_TICK
            1 -> HapticFeedbackConstants.KEYBOARD_TAP
            2 -> HapticFeedbackConstants.LONG_PRESS
            3 -> if (Build.VERSION.SDK_INT >= 30) HapticFeedbackConstants.CONFIRM else HapticFeedbackConstants.LONG_PRESS
            else -> if (Build.VERSION.SDK_INT >= 30) HapticFeedbackConstants.REJECT else HapticFeedbackConstants.LONG_PRESS
        }
        view.post { view.performHapticFeedback(c) }
    }

    override fun requestText(prompt: String?, current: String?): Boolean {
        activity.runOnUiThread {
            val field = EditText(activity).apply {
                setText(current ?: "")
                inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_CAP_WORDS
                setSingleLine()
            }
            AlertDialog.Builder(activity)
                .setTitle("Lemon Drop")
                .setMessage(prompt)
                .setView(field)
                .setNegativeButton("Cancel", null)
                .setPositiveButton("Save") { _, _ -> Mobile.textEntered(field.text.toString()) }
                .show()
        }
        return true
    }
}
