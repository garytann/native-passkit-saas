# Native iOS PassKit Client Flow

The backend creates and signs the `.pkpass`. The iOS app only downloads it and presents Apple's add-to-Wallet UI.

```swift
import PassKit
import UIKit

final class WalletPassPresenter {
    func presentPass(from url: URL, in viewController: UIViewController) async throws {
        let (data, _) = try await URLSession.shared.data(from: url)
        let pass = try PKPass(data: data)

        guard PKAddPassesViewController.canAddPasses(),
              let addPassVC = PKAddPassesViewController(pass: pass) else {
            return
        }

        await MainActor.run {
            viewController.present(addPassVC, animated: true)
        }
    }
}
```

Notes:

- The user must approve adding the pass.
- You can also open the `.pkpass` URL in Safari for the first proof of concept.
- Keep Apple certificate private keys on the backend only.
