import UIKit
import Mobile

/// Hosts the Ebitengine view (Metal) and passes the safe area to the game.
final class GameViewController: MobileEbitenViewController {
    lazy var bridge = NativeBridge(host: self)

    override var prefersStatusBarHidden: Bool { true }
    override var prefersHomeIndicatorAutoHidden: Bool { true }
    override var preferredScreenEdgesDeferringSystemGestures: UIRectEdge { .bottom }
    override var supportedInterfaceOrientations: UIInterfaceOrientationMask { .portrait }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        let i = view.safeAreaInsets
        MobileSetSafeArea(Double(i.top), Double(i.left), Double(i.bottom), Double(i.right))
    }

    override func onError(onGameUpdate err: Error) {
        NSLog("Lemon Drop: game error: %@", err.localizedDescription)
    }
}

/// Haptics and the name prompt, called from Go on the game's thread.
final class NativeBridge: NSObject, MobileBridgeProtocol {
    private weak var host: UIViewController?
    private let light = UIImpactFeedbackGenerator(style: .light)
    private let medium = UIImpactFeedbackGenerator(style: .medium)
    private let heavy = UIImpactFeedbackGenerator(style: .heavy)
    private let notify = UINotificationFeedbackGenerator()

    init(host: UIViewController) { self.host = host }

    func haptic(_ kind: Int) {
        DispatchQueue.main.async {
            switch kind {
            case 0: self.light.impactOccurred()
            case 1: self.medium.impactOccurred()
            case 2: self.heavy.impactOccurred()
            case 3: self.notify.notificationOccurred(.success)
            default: self.notify.notificationOccurred(.error)
            }
        }
    }

    func requestText(_ prompt: String?, current: String?) -> Bool {
        DispatchQueue.main.async {
            guard let host = self.host else { return }
            let alert = UIAlertController(title: "Lemon Drop", message: prompt, preferredStyle: .alert)
            alert.addTextField { field in
                field.text = current
                field.autocapitalizationType = .words
                field.autocorrectionType = .no
                field.returnKeyType = .done
            }
            alert.addAction(UIAlertAction(title: "Cancel", style: .cancel))
            alert.addAction(UIAlertAction(title: "Save", style: .default) { _ in
                MobileTextEntered(alert.textFields?.first?.text ?? "")
            })
            host.present(alert, animated: true)
        }
        return true
    }
}
