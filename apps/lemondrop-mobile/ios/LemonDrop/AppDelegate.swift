import UIKit
import Mobile

@main
final class AppDelegate: UIResponder, UIApplicationDelegate {
    func application(_ application: UIApplication,
                     didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        // Save data lives in Application Support (backed up, not user-visible).
        let dir = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        MobileSetDataDir(dir.path)
        return true
    }

    func application(_ application: UIApplication,
                     configurationForConnecting connectingSceneSession: UISceneSession,
                     options: UIScene.ConnectionOptions) -> UISceneConfiguration {
        UISceneConfiguration(name: "Default", sessionRole: connectingSceneSession.role)
    }
}

/// One window hosting the game; lifecycle goes to the Go side.
final class SceneDelegate: UIResponder, UIWindowSceneDelegate {
    var window: UIWindow?
    private var game: GameViewController? { window?.rootViewController as? GameViewController }

    func scene(_ scene: UIScene, willConnectTo session: UISceneSession,
               options connectionOptions: UIScene.ConnectionOptions) {
        guard let ws = scene as? UIWindowScene else { return }
        let vc = GameViewController()
        MobileSetBridge(vc.bridge)
        let w = UIWindow(windowScene: ws)
        w.rootViewController = vc
        w.makeKeyAndVisible()
        window = w
    }

    func sceneWillResignActive(_ scene: UIScene) {
        MobileSuspend()
        game?.suspendGame()
    }

    func sceneDidBecomeActive(_ scene: UIScene) {
        game?.resumeGame()
    }
}
