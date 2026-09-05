// SPDX-License-Identifier: AGPL-3.0-or-later
// The same shield and signal artwork as internal/panel/assets/favicon.svg.
import AppKit

guard CommandLine.arguments.count == 2 else { fatalError("expected .icns output path") }

func png(pixels: Int) -> Data {
    let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: pixels,
        pixelsHigh: pixels, bitsPerSample: 8, samplesPerPixel: 4,
        hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
        bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
    let transform = NSAffineTransform()
    transform.translateX(by: 0, yBy: CGFloat(pixels))
    transform.scaleX(by: CGFloat(pixels) / 32, yBy: -CGFloat(pixels) / 32)
    transform.concat()
    NSColor(deviceRed: 29/255, green: 34/255, blue: 42/255, alpha: 1).setFill()
    NSBezierPath(roundedRect: NSRect(x: 0, y: 0, width: 32, height: 32),
                 xRadius: 7, yRadius: 7).fill()
    let green = NSColor(deviceRed: 79/255, green: 189/255, blue: 133/255, alpha: 1)
    green.setStroke()
    let shield = NSBezierPath()
    shield.move(to: NSPoint(x: 16, y: 5))
    shield.line(to: NSPoint(x: 25, y: 9))
    shield.line(to: NSPoint(x: 25, y: 16))
    shield.curve(to: NSPoint(x: 16, y: 27.4), controlPoint1: NSPoint(x: 25, y: 21.5), controlPoint2: NSPoint(x: 21.3, y: 25.6))
    shield.curve(to: NSPoint(x: 7, y: 16), controlPoint1: NSPoint(x: 10.7, y: 25.6), controlPoint2: NSPoint(x: 7, y: 21.5))
    shield.line(to: NSPoint(x: 7, y: 9))
    shield.close()
    shield.lineWidth = 2.2
    shield.lineJoinStyle = .round
    shield.stroke()
    let signal = NSBezierPath()
    signal.move(to: NSPoint(x: 11.5, y: 16.5))
    signal.curve(to: NSPoint(x: 20.5, y: 16.5), controlPoint1: NSPoint(x: 14, y: 14), controlPoint2: NSPoint(x: 18, y: 14))
    signal.lineWidth = 2
    signal.lineCapStyle = .round
    signal.stroke()
    green.setFill()
    NSBezierPath(ovalIn: NSRect(x: 14.3, y: 18.7, width: 3.4, height: 3.4)).fill()
    NSGraphicsContext.restoreGraphicsState()
    return bitmap.representation(using: .png, properties: [:])!
}

func appendBigEndian(_ value: UInt32, to data: inout Data) {
    var encoded = value.bigEndian
    withUnsafeBytes(of: &encoded) { data.append(contentsOf: $0) }
}

// Modern ICNS entries are PNG payloads. Writing the small container directly
// avoids iconutil's macOS-version-dependent iconset validation while retaining
// every standard size from 16 through 1024 pixels.
let entries: [(String, Int)] = [
    ("icp4", 16), ("icp5", 32), ("icp6", 64), ("ic07", 128),
    ("ic08", 256), ("ic09", 512), ("ic10", 1024)
]
var body = Data()
for (type, pixels) in entries {
    let payload = png(pixels: pixels)
    body.append(contentsOf: type.utf8)
    appendBigEndian(UInt32(payload.count + 8), to: &body)
    body.append(payload)
}
var icon = Data("icns".utf8)
appendBigEndian(UInt32(body.count + 8), to: &icon)
icon.append(body)
try icon.write(to: URL(fileURLWithPath: CommandLine.arguments[1]), options: .atomic)
