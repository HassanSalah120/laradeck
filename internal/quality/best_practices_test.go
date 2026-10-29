package quality

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFullStackBestPracticesAudit_PHPAndInertiaV2(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hwb-test-project-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create controller directory and file
	ctrlDir := filepath.Join(tempDir, "app", "Http", "Controllers")
	if err := os.MkdirAll(ctrlDir, 0755); err != nil {
		t.Fatalf("failed to create controller dir: %v", err)
	}

	ctrlCode := `<?php
namespace App\Http\Controllers;

use App\Models\Order;
use Inertia\Inertia;

class OrderController
{
    public function index()
    {
        $secret = env('APP_SECRET');
        return Inertia::render('Orders/Index', [
            'orders' => Order::paginate(),
            'status' => 'active',
            'heavy' => Inertia::lazy(fn() => Order::count()),
        ]);
    }
}
`
	if err := os.WriteFile(filepath.Join(ctrlDir, "OrderController.php"), []byte(ctrlCode), 0644); err != nil {
		t.Fatalf("failed to write controller: %v", err)
	}

	// Create model with empty guarded
	modelsDir := filepath.Join(tempDir, "app", "Models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		t.Fatalf("failed to create models dir: %v", err)
	}
	modelCode := `<?php
namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Order extends Model
{
    protected $guarded = [];
}
`
	if err := os.WriteFile(filepath.Join(modelsDir, "Order.php"), []byte(modelCode), 0644); err != nil {
		t.Fatalf("failed to write model: %v", err)
	}

	// Create queue job missing attributes and timeout
	jobsDir := filepath.Join(tempDir, "app", "Jobs")
	if err := os.MkdirAll(jobsDir, 0755); err != nil {
		t.Fatalf("failed to create jobs dir: %v", err)
	}
	jobCode := `<?php
namespace App\Jobs;

use App\Models\Order;
use Illuminate\Contracts\Queue\ShouldQueue;

class ProcessOrder implements ShouldQueue
{
    public function __construct(public Order $order)
    {
    }

    public function handle(): void
    {
    }
}
`
	if err := os.WriteFile(filepath.Join(jobsDir, "ProcessOrder.php"), []byte(jobCode), 0644); err != nil {
		t.Fatalf("failed to write job: %v", err)
	}

	pCtx := ProjectContext{
		ProjectPath:  tempDir,
		HasInertiaV2: true,
	}

	findings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("RunFullStackBestPracticesAudit failed: %v", err)
	}

	foundRules := make(map[string]bool)
	for _, f := range findings {
		foundRules[f.Rule] = true
	}

	expectedRules := []string{
		"NoEnvOutsideConfig",
		"EmptyGuardedRisk",
		"WrapInertiaPropsInClosures",
		"VersionAwareLazyProps",
		"JobRetriesTimeout",
		"QueueJobWithoutRelations",
	}

	for _, rule := range expectedRules {
		if !foundRules[rule] {
			t.Errorf("expected rule %q to be detected, but it was not found among %v", rule, foundRules)
		}
	}
}

func TestFullStackBestPracticesAudit_InertiaV1Suppression(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hwb-test-v1-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctrlDir := filepath.Join(tempDir, "app", "Http", "Controllers")
	if err := os.MkdirAll(ctrlDir, 0755); err != nil {
		t.Fatalf("failed to create controller dir: %v", err)
	}

	ctrlCode := `<?php
namespace App\Http\Controllers;

use Inertia\Inertia;

class DashboardController
{
    public function show()
    {
        return Inertia::render('Dashboard', [
            'lazyData' => Inertia::lazy(fn() => [1, 2, 3]),
        ]);
    }
}
`
	if err := os.WriteFile(filepath.Join(ctrlDir, "DashboardController.php"), []byte(ctrlCode), 0644); err != nil {
		t.Fatalf("failed to write controller: %v", err)
	}

	pCtx := ProjectContext{
		ProjectPath:  tempDir,
		HasInertiaV2: false, // Inertia v1
	}

	findings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("RunFullStackBestPracticesAudit failed: %v", err)
	}

	for _, f := range findings {
		if f.Rule == "VersionAwareLazyProps" {
			t.Errorf("VersionAwareLazyProps should NOT trigger on Inertia v1 projects")
		}
	}
}

func TestBladeAndStructureConventions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hwb-test-blade-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Blade with queries
	viewsDir := filepath.Join(tempDir, "resources", "views")
	if err := os.MkdirAll(viewsDir, 0755); err != nil {
		t.Fatalf("failed to create views dir: %v", err)
	}
	bladeCode := `@foreach (App\Models\Order::all() as $order)
    <div>{{ $order->id }}</div>
@endforeach
`
	if err := os.WriteFile(filepath.Join(viewsDir, "orders.blade.php"), []byte(bladeCode), 0644); err != nil {
		t.Fatalf("failed to write blade view: %v", err)
	}

	// Plural Model and Controller names
	modelsDir := filepath.Join(tempDir, "app", "Models")
	ctrlDir := filepath.Join(tempDir, "app", "Http", "Controllers")
	_ = os.MkdirAll(modelsDir, 0755)
	_ = os.MkdirAll(ctrlDir, 0755)

	_ = os.WriteFile(filepath.Join(modelsDir, "Users.php"), []byte("<?php class Users {}"), 0644)
	_ = os.WriteFile(filepath.Join(ctrlDir, "OrdersController.php"), []byte("<?php class OrdersController {}"), 0644)

	findings, scanErr := ScanBladeAndStructure(tempDir)
	if scanErr != nil {
		t.Fatal(scanErr)
	}
	foundRules := make(map[string]bool)
	for _, f := range findings {
		foundRules[f.Rule] = true
	}

	if !foundRules["NoQueriesInBlade"] {
		t.Errorf("expected NoQueriesInBlade to be flagged")
	}
	if !foundRules["ModelNamingSingular"] {
		t.Errorf("expected ModelNamingSingular to be flagged for Users.php")
	}
	if !foundRules["ControllerNamingSingular"] {
		t.Errorf("expected ControllerNamingSingular to be flagged for OrdersController.php")
	}
}

func TestTSXAndReactScanner_EdgeCasesAndEchoSuppression(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hwb-test-tsx-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	jsDir := filepath.Join(tempDir, "resources", "js", "Pages")
	if err := os.MkdirAll(jsDir, 0755); err != nil {
		t.Fatalf("failed to create js dir: %v", err)
	}

	// Violation component
	badCode := `import React, { useEffect, useState } from 'react';
import { usePage, useForm } from '@inertiajs/react';

export default function OrderView({ condition }) {
    if (condition) {
        const [sub, setSub] = useState(0); // RulesOfHooks violation
    }

    const { auth } = usePage().props; // Untyped usePage violation
    const form = useForm({ avatar: null as File | null });

    const handleUpload = () => {
        form.put('/profile'); // InertiaFilePutSpoofing violation
    };

    const handleInput = (e: any) => {
        channel.whisper('typing', { user: 'me' }); // UnthrottledClientWhisper violation
    };

    useEffect(() => {
        Echo.private('orders').listen('OrderShipped', (e) => {
            setItems([...items, e]); // Stale closure violation
        });
        // Missing cleanup return -> EchoCleanupUnmount violation
    }, []);

    return (
        <div>
            {items.map((it, index) => (
                <div key={index}>{it.title}</div>
            ))}
        </div>
    );
}
`
	if err := os.WriteFile(filepath.Join(jsDir, "OrderView.tsx"), []byte(badCode), 0644); err != nil {
		t.Fatalf("failed to write bad tsx: %v", err)
	}

	// Declarative safe component using useEcho() hook from @laravel/echo-react
	safeCode := `import React from 'react';
import { usePage } from '@inertiajs/react';
import { useEcho } from '@laravel/echo-react';

interface PageProps {
    auth: { user: { id: number } };
}

export default function SafeView() {
    const { auth } = usePage<PageProps>().props;

    useEcho('orders', 'OrderUpdated', (e) => {
        console.log(e);
    });

    return <div>Safe</div>;
}
`
	if err := os.WriteFile(filepath.Join(jsDir, "SafeView.tsx"), []byte(safeCode), 0644); err != nil {
		t.Fatalf("failed to write safe tsx: %v", err)
	}

	findings, scanErr := ScanTSXAndReact(tempDir)
	if scanErr != nil {
		t.Fatal(scanErr)
	}
	foundOnOrderView := make(map[string]bool)
	foundOnSafeView := make(map[string]bool)

	for _, f := range findings {
		if filepath.Base(f.File) == "OrderView.tsx" {
			foundOnOrderView[f.Rule] = true
		}
		if filepath.Base(f.File) == "SafeView.tsx" {
			foundOnSafeView[f.Rule] = true
		}
	}

	expectedViolations := []string{
		"RulesOfHooks",
		"TypedInertiaPageProps",
		"InertiaFilePutSpoofing",
		"EchoCleanupUnmount",
		"StaleClosureInSocketListener",
		"StableListKeys",
	}

	for _, rule := range expectedViolations {
		if !foundOnOrderView[rule] {
			t.Errorf("expected %s on OrderView.tsx, but was not detected in %v", rule, foundOnOrderView)
		}
	}

	// Verify useEcho() from @laravel/echo-react in SafeView is NOT flagged for EchoCleanupUnmount
	if foundOnSafeView["EchoCleanupUnmount"] {
		t.Errorf("Constraint 4 violation: useEcho() from @laravel/echo-react was erroneously flagged by EchoCleanupUnmount")
	}
	if foundOnSafeView["TypedInertiaPageProps"] {
		t.Errorf("Typed usePage<PageProps>() was erroneously flagged by TypedInertiaPageProps")
	}
}

func TestFalsePositiveSuppression_5EdgeCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hwb-test-suppressions-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Edge Case 1: NoMutationsInRoutes false-positive on Route::delete / Route::post
	routesDir := filepath.Join(tempDir, "routes")
	_ = os.MkdirAll(routesDir, 0755)
	safeRouteCode := `<?php
use App\Http\Controllers\CampaignController;
use App\Http\Controllers\UserController;
use Illuminate\Support\Facades\Route;

Route::get('/campaigns', [CampaignController::class, 'index']);
Route::delete('/campaigns/{id}', [CampaignController::class, 'destroy']);
Route::post('/users', [UserController::class, 'store']);
`
	if err := os.WriteFile(filepath.Join(routesDir, "campaigns.php"), []byte(safeRouteCode), 0644); err != nil {
		t.Fatalf("failed to write safe routes: %v", err)
	}

	badRouteCode := `<?php
use App\Models\User;
use Illuminate\Support\Facades\Route;

Route::delete('/inline-delete', function () {
    User::delete();
});
`
	if err := os.WriteFile(filepath.Join(routesDir, "mutations.php"), []byte(badRouteCode), 0644); err != nil {
		t.Fatalf("failed to write bad routes: %v", err)
	}

	// 2. Edge Case 2: LightSharedProps false-positive on session flash messages, config, cache
	mwDir := filepath.Join(tempDir, "app", "Http", "Middleware")
	_ = os.MkdirAll(mwDir, 0755)
	safeMwCode := `<?php
namespace App\Http\Middleware;

use Illuminate\Http\Request;
use Inertia\Middleware;
use Illuminate\Support\Facades\Cache;

class HandleInertiaRequests extends Middleware
{
    public function share(Request $request): array
    {
        return array_merge(parent::share($request), [
            'flash' => [
                'success' => $request->session()->get('success'),
                'error' => session()->get('error'),
            ],
            'csrf' => $request->headers->get('X-CSRF-TOKEN'),
            'cached' => Cache::get('site_key'),
            'appName' => config('app.name'),
        ]);
    }
}
`
	if err := os.WriteFile(filepath.Join(mwDir, "HandleInertiaRequests.php"), []byte(safeMwCode), 0644); err != nil {
		t.Fatalf("failed to write safe middleware: %v", err)
	}

	badMwCode := `<?php
namespace App\Http\Middleware;

use App\Models\User;
use Illuminate\Http\Request;
use Inertia\Middleware;

class HandleInertiaRequestsHeavy extends Middleware
{
    public function share(Request $request): array
    {
        return [
            'users' => User::all(),
        ];
    }
}
`
	if err := os.WriteFile(filepath.Join(mwDir, "HandleInertiaRequestsHeavy.php"), []byte(badMwCode), 0644); err != nil {
		t.Fatalf("failed to write bad middleware: %v", err)
	}

	// 3. Edge Case 3: QueueJobWithoutRelations scalar parameters & WithoutRelations attribute
	jobsDir := filepath.Join(tempDir, "app", "Jobs")
	_ = os.MkdirAll(jobsDir, 0755)
	scalarJobCode := `<?php
namespace App\Jobs;

use Illuminate\Contracts\Queue\ShouldQueue;

class BackupJob implements ShouldQueue
{
    public int $tries = 3;
    public int $timeout = 120;

    public function __construct(
        public int $backupId,
        public string $disk,
        public ?array $options = null
    ) {
    }
}
`
	if err := os.WriteFile(filepath.Join(jobsDir, "BackupJob.php"), []byte(scalarJobCode), 0644); err != nil {
		t.Fatalf("failed to write scalar job: %v", err)
	}

	annotatedJobCode := `<?php
namespace App\Jobs;

use App\Models\Order;
use Illuminate\Queue\Attributes\WithoutRelations;
use Illuminate\Contracts\Queue\ShouldQueue;

class SafeOrderJob implements ShouldQueue
{
    public int $tries = 3;
    public int $timeout = 60;

    public function __construct(
        #[WithoutRelations]
        public Order $order
    ) {
    }
}
`
	if err := os.WriteFile(filepath.Join(jobsDir, "SafeOrderJob.php"), []byte(annotatedJobCode), 0644); err != nil {
		t.Fatalf("failed to write annotated job: %v", err)
	}

	unsafeJobCode := `<?php
namespace App\Jobs;

use App\Models\Order;
use Illuminate\Contracts\Queue\ShouldQueue;

class UnsafeOrderJob implements ShouldQueue
{
    public int $tries = 3;
    public int $timeout = 60;

    public function __construct(
        public Order $order
    ) {
    }
}
`
	if err := os.WriteFile(filepath.Join(jobsDir, "UnsafeOrderJob.php"), []byte(unsafeJobCode), 0644); err != nil {
		t.Fatalf("failed to write unsafe job: %v", err)
	}

	// 4 & 5. Edge Cases 4 & 5 in TSX: StableListKeys skeleton suppression & TypedInertiaPageProps url destructuring
	jsDir := filepath.Join(tempDir, "resources", "js", "Components")
	_ = os.MkdirAll(jsDir, 0755)

	safeTsxCode := `import React from 'react';
import { usePage } from '@inertiajs/react';

export default function SkeletonList() {
    // Edge case 5: url destructuring from usePage() without props access
    const { url, component } = usePage();

    return (
        <div className="space-y-2">
            {/* Edge case 4: loading skeleton with animate-pulse */}
            {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="h-4 bg-gray-200 animate-pulse rounded" />
            ))}
            {[...Array(3)].map((_, idx) => (
                <div key={idx} className="skeleton h-8 w-full" />
            ))}
        </div>
    );
}
`
	if err := os.WriteFile(filepath.Join(jsDir, "SkeletonList.tsx"), []byte(safeTsxCode), 0644); err != nil {
		t.Fatalf("failed to write safe tsx: %v", err)
	}

	unsafeTsxCode := `import React from 'react';
import { usePage } from '@inertiajs/react';

export default function ItemList({ items }: { items: any[] }) {
    // Untyped props access
    const { auth } = usePage().props;

    return (
        <div>
            {items.map((item, index) => (
                <div key={index}>{item.title}</div>
            ))}
        </div>
    );
}
`
	if err := os.WriteFile(filepath.Join(jsDir, "ItemList.tsx"), []byte(unsafeTsxCode), 0644); err != nil {
		t.Fatalf("failed to write unsafe tsx: %v", err)
	}

	// Run PHP scan
	pCtx := ProjectContext{
		ProjectPath:  tempDir,
		HasInertiaV2: true,
	}
	phpFindings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("RunFullStackBestPracticesAudit failed: %v", err)
	}

	for _, f := range phpFindings {
		rel := filepath.ToSlash(f.File)

		// 1. campaigns.php must NOT have NoMutationsInRoutes
		if strings.Contains(rel, "campaigns.php") && f.Rule == "NoMutationsInRoutes" {
			t.Errorf("Edge case 1 failure: Route::delete in campaigns.php was wrongly flagged by NoMutationsInRoutes: %s", f.Snippet)
		}

		// 2. HandleInertiaRequests.php must NOT have LightSharedProps
		if strings.Contains(rel, "HandleInertiaRequests.php") && f.Rule == "LightSharedProps" {
			t.Errorf("Edge case 2 failure: session/cache/config read in HandleInertiaRequests.php was wrongly flagged by LightSharedProps: %s", f.Snippet)
		}

		// 3. BackupJob and SafeOrderJob must NOT have QueueJobWithoutRelations
		if strings.Contains(rel, "BackupJob.php") && f.Rule == "QueueJobWithoutRelations" {
			t.Errorf("Edge case 3 failure: scalar parameters in BackupJob.php were wrongly flagged by QueueJobWithoutRelations")
		}
		if strings.Contains(rel, "SafeOrderJob.php") && f.Rule == "QueueJobWithoutRelations" {
			t.Errorf("Edge case 3 failure: #[WithoutRelations] in SafeOrderJob.php was wrongly flagged by QueueJobWithoutRelations")
		}

		// Verify line number accuracy on UnsafeOrderJob
		if strings.Contains(rel, "UnsafeOrderJob.php") && f.Rule == "QueueJobWithoutRelations" {
			if f.Line <= 1 {
				t.Errorf("Edge case 3 failure: UnsafeOrderJob line number is %d, expected line > 1 (the constructor parameter line)", f.Line)
			}
			if !strings.Contains(f.Snippet, "$order") {
				t.Errorf("Edge case 3 failure: UnsafeOrderJob snippet %q does not contain $order", f.Snippet)
			}
		}
	}

	// Verify true positives were caught in PHP
	foundMutationsInRoutes := false
	foundHeavySharedProps := false
	foundUnsafeOrderJob := false
	for _, f := range phpFindings {
		rel := filepath.ToSlash(f.File)
		if strings.Contains(rel, "mutations.php") && f.Rule == "NoMutationsInRoutes" {
			foundMutationsInRoutes = true
		}
		if strings.Contains(rel, "HandleInertiaRequestsHeavy.php") && f.Rule == "LightSharedProps" {
			foundHeavySharedProps = true
		}
		if strings.Contains(rel, "UnsafeOrderJob.php") && f.Rule == "QueueJobWithoutRelations" {
			foundUnsafeOrderJob = true
		}
	}

	if !foundMutationsInRoutes {
		t.Errorf("Expected mutations.php to be flagged by NoMutationsInRoutes")
	}
	if !foundHeavySharedProps {
		t.Errorf("Expected HandleInertiaRequestsHeavy.php to be flagged by LightSharedProps")
	}
	if !foundUnsafeOrderJob {
		t.Errorf("Expected UnsafeOrderJob.php to be flagged by QueueJobWithoutRelations")
	}

	// Run TSX scan
	tsxFindings, scanErr := ScanTSXAndReact(tempDir)
	if scanErr != nil {
		t.Fatal(scanErr)
	}
	for _, f := range tsxFindings {
		rel := filepath.ToSlash(f.File)

		// 4. SkeletonList.tsx must NOT have StableListKeys
		if strings.Contains(rel, "SkeletonList.tsx") && f.Rule == "StableListKeys" {
			t.Errorf("Edge case 4 failure: SkeletonList.tsx was wrongly flagged by StableListKeys: %s", f.Snippet)
		}

		// 5. SkeletonList.tsx must NOT have TypedInertiaPageProps
		if strings.Contains(rel, "SkeletonList.tsx") && f.Rule == "TypedInertiaPageProps" {
			t.Errorf("Edge case 5 failure: const { url } = usePage() in SkeletonList.tsx was wrongly flagged by TypedInertiaPageProps: %s", f.Snippet)
		}
	}

	// Verify true positives in TSX
	foundItemListStableKeys := false
	foundItemListTypedProps := false
	for _, f := range tsxFindings {
		rel := filepath.ToSlash(f.File)
		if strings.Contains(rel, "ItemList.tsx") && f.Rule == "StableListKeys" {
			foundItemListStableKeys = true
			if f.Severity != "info" {
				t.Errorf("Edge case 4 failure: StableListKeys severity should be 'info', got %q", f.Severity)
			}
		}
		if strings.Contains(rel, "ItemList.tsx") && f.Rule == "TypedInertiaPageProps" {
			foundItemListTypedProps = true
		}
	}

	if !foundItemListStableKeys {
		t.Errorf("Expected ItemList.tsx to be flagged by StableListKeys")
	}
	if !foundItemListTypedProps {
		t.Errorf("Expected ItemList.tsx to be flagged by TypedInertiaPageProps")
	}
}

func TestJobRetriesTimeout_ScopeAndClassConstantPolish(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hwb-test-polish-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Mailable in app/Mail/
	mailDir := filepath.Join(tempDir, "app", "Mail")
	_ = os.MkdirAll(mailDir, 0755)
	mailCode := `<?php
namespace App\Mail;

use Illuminate\Contracts\Queue\ShouldQueue;

class ClinicWorkspaceProvisionedMail implements ShouldQueue
{
    public function build()
    {
        $service = app(ClinicSubscriptionCommunicationService::class);
        return $this->view('emails.provisioned');
    }
}
`
	if err := os.WriteFile(filepath.Join(mailDir, "ClinicWorkspaceProvisionedMail.php"), []byte(mailCode), 0644); err != nil {
		t.Fatalf("failed to write mailable: %v", err)
	}

	// 2. Notification in app/Notifications/
	notifDir := filepath.Join(tempDir, "app", "Notifications")
	_ = os.MkdirAll(notifDir, 0755)
	notifCode := `<?php
namespace App\Notifications;

use Illuminate\Contracts\Queue\ShouldQueue;

class AppointmentReminderNotification implements ShouldQueue
{
    public function via($notifiable)
    {
        $service = app(CommunicationTemplateServiceInterface::class);
        return ['mail'];
    }
}
`
	if err := os.WriteFile(filepath.Join(notifDir, "AppointmentReminderNotification.php"), []byte(notifCode), 0644); err != nil {
		t.Fatalf("failed to write notification: %v", err)
	}

	// 3. Abstract Job in app/Jobs/
	jobsDir := filepath.Join(tempDir, "app", "Jobs")
	_ = os.MkdirAll(jobsDir, 0755)
	abstractJobCode := `<?php
namespace App\Jobs;

use Illuminate\Contracts\Queue\ShouldQueue;

abstract class BaseClinicJob implements ShouldQueue
{
    public function handle(): void
    {
    }
}
`
	if err := os.WriteFile(filepath.Join(jobsDir, "BaseClinicJob.php"), []byte(abstractJobCode), 0644); err != nil {
		t.Fatalf("failed to write abstract job: %v", err)
	}

	// 4. Concrete Job in app/Jobs/ containing ::class resolution in method
	concreteJobCode := `<?php
namespace App\Jobs;

use Illuminate\Contracts\Queue\ShouldQueue;

class SendReceiptJob implements ShouldQueue
{
    public function handle(): void
    {
        $dummy1 = 1;
        $dummy2 = 2;
        $service = app(ReceiptPrinterService::class);
    }
}
`
	if err := os.WriteFile(filepath.Join(jobsDir, "SendReceiptJob.php"), []byte(concreteJobCode), 0644); err != nil {
		t.Fatalf("failed to write concrete job: %v", err)
	}

	pCtx := ProjectContext{
		ProjectPath: tempDir,
	}
	findings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("RunFullStackBestPracticesAudit failed: %v", err)
	}

	for _, f := range findings {
		rel := filepath.ToSlash(f.File)

		// Mailables must NOT be flagged for JobRetriesTimeout
		if strings.Contains(rel, "ClinicWorkspaceProvisionedMail.php") && f.Rule == "JobRetriesTimeout" {
			t.Errorf("Mailable ClinicWorkspaceProvisionedMail.php was erroneously flagged by JobRetriesTimeout: line %d", f.Line)
		}

		// Notifications must NOT be flagged for JobRetriesTimeout
		if strings.Contains(rel, "AppointmentReminderNotification.php") && f.Rule == "JobRetriesTimeout" {
			t.Errorf("Notification AppointmentReminderNotification.php was erroneously flagged by JobRetriesTimeout: line %d", f.Line)
		}

		// Abstract jobs must NOT be flagged for JobRetriesTimeout
		if strings.Contains(rel, "BaseClinicJob.php") && f.Rule == "JobRetriesTimeout" {
			t.Errorf("Abstract class BaseClinicJob.php was erroneously flagged by JobRetriesTimeout: line %d", f.Line)
		}
	}

	// Concrete job MUST be flagged, AND its line must be the class declaration line (line 6), NOT line 12 where ReceiptPrinterService::class occurs
	foundConcreteJob := false
	for _, f := range findings {
		rel := filepath.ToSlash(f.File)
		if strings.Contains(rel, "SendReceiptJob.php") && f.Rule == "JobRetriesTimeout" {
			foundConcreteJob = true
			if f.Line != 6 {
				t.Errorf("SendReceiptJob line number was %d, expected line 6 (class declaration line), was overwritten by ::class", f.Line)
			}
			if !strings.Contains(f.Snippet, "class SendReceiptJob") {
				t.Errorf("SendReceiptJob snippet %q was expected to contain 'class SendReceiptJob', but had ::class snippet", f.Snippet)
			}
		}
	}

	if !foundConcreteJob {
		t.Errorf("Expected SendReceiptJob to be flagged by JobRetriesTimeout")
	}
}

func TestScannerHardening_QueueJobAndMarketingSuppression(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Setup PHP directory structure
	mailDir := filepath.Join(tempDir, "app", "Mail")
	jobsDir := filepath.Join(tempDir, "app", "Jobs")
	_ = os.MkdirAll(mailDir, 0755)
	_ = os.MkdirAll(jobsDir, 0755)

	// A: Synchronous Mail without ShouldQueue - MUST NOT be flagged
	syncMailCode := `<?php
namespace App\Mail;

use App\Models\BookingRequest;
use Illuminate\Mail\Mailable;

class SyncBookingMail extends Mailable
{
    public function __construct(public BookingRequest $bookingRequest)
    {
    }
}
`
	if err := os.WriteFile(filepath.Join(mailDir, "SyncBookingMail.php"), []byte(syncMailCode), 0644); err != nil {
		t.Fatalf("failed to write sync mail: %v", err)
	}

	// B: Queued Mail with ShouldQueue and unprotected model - MUST be flagged
	queuedMailCode := `<?php
namespace App\Mail;

use App\Models\BookingRequest;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Mail\Mailable;

class QueuedBookingMail extends Mailable implements ShouldQueue
{
    public function __construct(public BookingRequest $bookingRequest)
    {
    }
}
`
	if err := os.WriteFile(filepath.Join(mailDir, "QueuedBookingMail.php"), []byte(queuedMailCode), 0644); err != nil {
		t.Fatalf("failed to write queued mail: %v", err)
	}

	// C: Queued Job with DTO & Enum params - MUST NOT be flagged
	dtoJobCode := `<?php
namespace App\Jobs;

use App\DTOs\PatientTransferDTO;
use App\Enums\TransferStatusEnum;
use Illuminate\Contracts\Queue\ShouldQueue;

class TransferPatientJob implements ShouldQueue
{
    public int $tries = 3;
    public int $timeout = 60;

    public function __construct(
        public PatientTransferDTO $transferDto,
        public TransferStatusEnum $status
    ) {
    }
}
`
	if err := os.WriteFile(filepath.Join(jobsDir, "TransferPatientJob.php"), []byte(dtoJobCode), 0644); err != nil {
		t.Fatalf("failed to write dto job: %v", err)
	}

	// 2. Setup React TSX directory structure
	marketingDir := filepath.Join(tempDir, "resources", "js", "Pages", "Marketing")
	appDir := filepath.Join(tempDir, "resources", "js", "Pages", "App")
	_ = os.MkdirAll(marketingDir, 0755)
	_ = os.MkdirAll(appDir, 0755)

	marketingCode := `import React from 'react';
export default function FeatureDetail() {
    const features = ['Fast', 'Reliable', 'Secure'];
    return (
        <div>
            {features.map((item, index) => (
                <div key={index}>{item}</div>
            ))}
        </div>
    );
}
`
	if err := os.WriteFile(filepath.Join(marketingDir, "FeatureDetail.tsx"), []byte(marketingCode), 0644); err != nil {
		t.Fatalf("failed to write marketing tsx: %v", err)
	}

	interactiveCode := `import React, { useState } from 'react';
export default function ClinicalAssistant() {
    const [notes, setNotes] = useState(['Note 1', 'Note 2']);
    return (
        <div>
            {notes.map((note, index) => (
                <li key={index}>{note}</li>
            ))}
        </div>
    );
}
`
	if err := os.WriteFile(filepath.Join(appDir, "ClinicalAssistant.tsx"), []byte(interactiveCode), 0644); err != nil {
		t.Fatalf("failed to write interactive tsx: %v", err)
	}

	// Run full-stack scanner
	pCtx := ProjectContext{ProjectPath: tempDir}
	findings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("scanBuiltinsRaw failed: %v", err)
	}

	foundQueuedMail := false
	foundInteractiveTsx := false

	for _, f := range findings {
		rel := filepath.ToSlash(f.File)

		// Synchronous mail must NOT be flagged
		if strings.Contains(rel, "SyncBookingMail.php") && f.Rule == "QueueJobWithoutRelations" {
			t.Errorf("SyncBookingMail.php was erroneously flagged for QueueJobWithoutRelations despite not implementing ShouldQueue")
		}

		// Queued mail MUST be flagged
		if strings.Contains(rel, "QueuedBookingMail.php") && f.Rule == "QueueJobWithoutRelations" {
			foundQueuedMail = true
		}

		// Job with DTO/Enum must NOT be flagged
		if strings.Contains(rel, "TransferPatientJob.php") && f.Rule == "QueueJobWithoutRelations" {
			t.Errorf("TransferPatientJob.php was erroneously flagged for QueueJobWithoutRelations on DTO/Enum parameter")
		}

		// Marketing TSX must NOT be flagged
		if strings.Contains(rel, "FeatureDetail.tsx") && f.Rule == "StableListKeys" {
			t.Errorf("Marketing page FeatureDetail.tsx was erroneously flagged for StableListKeys")
		}

		// Interactive TSX MUST be flagged
		if strings.Contains(rel, "ClinicalAssistant.tsx") && f.Rule == "StableListKeys" {
			foundInteractiveTsx = true
		}
	}

	if !foundQueuedMail {
		t.Errorf("Expected QueuedBookingMail.php to be flagged for QueueJobWithoutRelations")
	}
	if !foundInteractiveTsx {
		t.Errorf("Expected ClinicalAssistant.tsx to be flagged for StableListKeys")
	}
}

func TestBpdHighSignalRules(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Controller MultiWrite, Upload, Redirect, Registration
	ctrlDir := filepath.Join(tempDir, "app", "Http", "Controllers")
	_ = os.MkdirAll(ctrlDir, 0755)

	orderCtrlCode := `<?php
namespace App\Http\Controllers;

use App\Models\Order;
use App\Models\Invoice;
use Illuminate\Support\Facades\DB;

class OrderController
{
    public function checkout()
    {
        $order = Order::create(['total' => 100]);
        $invoice = Invoice::create(['order_id' => $order->id]);
        return response()->json(['status' => 'ok']);
    }

    public function safeCheckout()
    {
        return DB::transaction(function () {
            $order = Order::create(['total' => 100]);
            $invoice = Invoice::create(['order_id' => $order->id]);
            return response()->json(['status' => 'ok']);
        });
    }
}
`
	_ = os.WriteFile(filepath.Join(ctrlDir, "OrderController.php"), []byte(orderCtrlCode), 0644)

	profileCtrlCode := `<?php
namespace App\Http\Controllers;

use Illuminate\Http\Request;

class ProfileController
{
    public function uploadAvatar(Request $request)
    {
        $path = $request->file('avatar')->store('avatars');
        return back();
    }
}
`
	_ = os.WriteFile(filepath.Join(ctrlDir, "ProfileController.php"), []byte(profileCtrlCode), 0644)

	redirectCtrlCode := `<?php
namespace App\Http\Controllers;

use Illuminate\Http\Request;

class RedirectController
{
    public function handle(Request $request)
    {
        $dest = $request->input('target');
        return redirect()->away($dest);
    }
}
`
	_ = os.WriteFile(filepath.Join(ctrlDir, "RedirectController.php"), []byte(redirectCtrlCode), 0644)

	regCtrlCode := `<?php
namespace App\Http\Controllers\Auth;

use App\Models\User;
use Illuminate\Http\Request;

class RegisteredUserController
{
    public function store(Request $request)
    {
        $user = User::create(['email' => $request->email]);
        return redirect('/dashboard');
    }
}
`
	_ = os.WriteFile(filepath.Join(ctrlDir, "RegisteredUserController.php"), []byte(regCtrlCode), 0644)

	// 2. Jobs Unbounded Query
	jobsDir := filepath.Join(tempDir, "app", "Jobs")
	_ = os.MkdirAll(jobsDir, 0755)

	jobCode := `<?php
namespace App\Jobs;

use App\Models\Invoice;
use Illuminate\Contracts\Queue\ShouldQueue;

class ExportInvoicesJob implements ShouldQueue
{
    public int $tries = 3;
    public int $timeout = 60;

    public function handle(): void
    {
        foreach (Invoice::all() as $invoice) {
            $this->process($invoice);
        }
    }
}
`
	_ = os.WriteFile(filepath.Join(jobsDir, "ExportInvoicesJob.php"), []byte(jobCode), 0644)

	// 3. Models Exposed Sensitive Attributes
	modelsDir := filepath.Join(tempDir, "app", "Models")
	_ = os.MkdirAll(modelsDir, 0755)

	modelCode := `<?php
namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class CustomerSecret extends Model
{
    protected $casts = [
        'password' => 'hashed',
        'api_token' => 'string',
    ];
}
`
	_ = os.WriteFile(filepath.Join(modelsDir, "CustomerSecret.php"), []byte(modelCode), 0644)

	// 4. Services Sensitive Data Logging & Unsafe Dynamic Execution
	servicesDir := filepath.Join(tempDir, "app", "Services")
	_ = os.MkdirAll(servicesDir, 0755)

	serviceCode := `<?php
namespace App\Services;

use Illuminate\Support\Facades\Log;

class AuditLogService
{
    public function logAttempt(string $password): void
    {
        Log::info('Login event', [
            'password' => $password,
        ]);
        eval('$foo = 1;');
    }
}
`
	_ = os.WriteFile(filepath.Join(servicesDir, "AuditLogService.php"), []byte(serviceCode), 0644)

	// 5. Routes Auth Throttle
	routesDir := filepath.Join(tempDir, "routes")
	_ = os.MkdirAll(routesDir, 0755)

	routesCode := `<?php
use Illuminate\Support\Facades\Route;

Route::post('/login', [LoginController::class, 'login']);
Route::post('/safe-login', [LoginController::class, 'login'])->middleware('throttle:6,1');
`
	_ = os.WriteFile(filepath.Join(routesDir, "web.php"), []byte(routesCode), 0644)

	// 6. TSX ReactEffectCleanupRequired & SafeTargetBlankRel
	pagesDir := filepath.Join(tempDir, "resources", "js", "Pages")
	_ = os.MkdirAll(pagesDir, 0755)

	tsxCode := `import React, { useEffect } from 'react';

export default function ExternalLink() {
    useEffect(() => {
        window.addEventListener('resize', () => {});
    }, []);

    return (
        <div>
            <a href="https://phishing.example.com" target="_blank">External</a>
            <a href="https://trusted.example.com" target="_blank" rel="noopener noreferrer">Safe</a>
        </div>
    );
}
`
	_ = os.WriteFile(filepath.Join(pagesDir, "ExternalLink.tsx"), []byte(tsxCode), 0644)

	// Run full analysis
	pCtx := ProjectContext{ProjectPath: tempDir}
	findings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("scanBuiltinsRaw failed: %v", err)
	}

	foundMultiWrite := false
	foundUpload := false
	foundRedirect := false
	foundRegistration := false
	foundUnbounded := false
	foundExposedAttr := false
	foundLogging := false
	foundEval := false
	foundThrottle := false
	foundEffectCleanup := false
	foundTargetBlank := false

	for _, f := range findings {
		rel := filepath.ToSlash(f.File)

		if strings.Contains(rel, "OrderController.php") && f.Rule == "MultiWriteTransactionRequired" {
			foundMultiWrite = true
			if f.Line > 18 {
				t.Errorf("MultiWriteTransactionRequired flagged safeCheckout() wrapped in DB::transaction: line %d", f.Line)
			}
		}
		if strings.Contains(rel, "ProfileController.php") && f.Rule == "UnsafeFileUploadValidation" {
			foundUpload = true
		}
		if strings.Contains(rel, "RedirectController.php") && f.Rule == "UnsafeExternalRedirect" {
			foundRedirect = true
		}
		if strings.Contains(rel, "RegisteredUserController.php") && f.Rule == "RegistrationRegisteredEvent" {
			foundRegistration = true
		}
		if strings.Contains(rel, "ExportInvoicesJob.php") && f.Rule == "UnboundedQueryInJobs" {
			foundUnbounded = true
		}
		if strings.Contains(rel, "CustomerSecret.php") && f.Rule == "ModelExposedSensitiveAttributes" {
			foundExposedAttr = true
		}
		if strings.Contains(rel, "AuditLogService.php") && f.Rule == "SensitiveDataLogging" {
			foundLogging = true
		}
		if strings.Contains(rel, "AuditLogService.php") && f.Rule == "UnsafeDynamicExecution" {
			foundEval = true
		}
		if strings.Contains(rel, "routes/web.php") && f.Rule == "AuthRouteRateLimiting" {
			foundThrottle = true
		}
		if strings.Contains(rel, "ExternalLink.tsx") && f.Rule == "ReactEffectCleanupRequired" {
			foundEffectCleanup = true
		}
		if strings.Contains(rel, "ExternalLink.tsx") && f.Rule == "SafeTargetBlankRel" {
			foundTargetBlank = true
		}
	}

	if !foundMultiWrite {
		t.Errorf("Expected MultiWriteTransactionRequired finding in OrderController.php")
	}
	if !foundUpload {
		t.Errorf("Expected UnsafeFileUploadValidation finding in ProfileController.php")
	}
	if !foundRedirect {
		t.Errorf("Expected UnsafeExternalRedirect finding in RedirectController.php")
	}
	if !foundRegistration {
		t.Errorf("Expected RegistrationRegisteredEvent finding in RegisteredUserController.php")
	}
	if !foundUnbounded {
		t.Errorf("Expected UnboundedQueryInJobs finding in ExportInvoicesJob.php")
	}
	if !foundExposedAttr {
		t.Errorf("Expected ModelExposedSensitiveAttributes finding in CustomerSecret.php")
	}
	if !foundLogging {
		t.Errorf("Expected SensitiveDataLogging finding in AuditLogService.php")
	}
	if !foundEval {
		t.Errorf("Expected UnsafeDynamicExecution finding in AuditLogService.php")
	}
	if !foundThrottle {
		t.Errorf("Expected AuthRouteRateLimiting finding in routes/web.php")
	}
	if !foundEffectCleanup {
		t.Errorf("Expected ReactEffectCleanupRequired finding in ExternalLink.tsx")
	}
	if !foundTargetBlank {
		t.Errorf("Expected SafeTargetBlankRel finding in ExternalLink.tsx")
	}
}

func TestQuizpokerFalsePositiveSuppression_WorkbenchHeuristic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "quizpoker-hardening-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	actionsDir := filepath.Join(tempDir, "app", "Actions", "QuizPoker")
	_ = os.MkdirAll(actionsDir, 0755)

	controllersDir := filepath.Join(tempDir, "app", "Http", "Controllers")
	_ = os.MkdirAll(controllersDir, 0755)

	routesDir := filepath.Join(tempDir, "routes")
	_ = os.MkdirAll(routesDir, 0755)

	// 1. BettingActions.php: Action with transaction orchestrator calling executeCall, executeRaise, etc.
	bettingActionsCode := `<?php
namespace App\Actions\QuizPoker;

use Illuminate\Support\Facades\DB;
use App\Models\Bet;
use App\Models\Player;

class BettingActions
{
    public function handlePlayerBet($match, $player, $action)
    {
        return DB::transaction(function () use ($match, $player, $action) {
            if ($action === 'call') {
                return $this->executeCall($match, $player);
            }
            if ($action === 'raise') {
                return $this->executeRaise($match, $player);
            }
            if ($action === 'allin') {
                return $this->executeAllIn($match, $player);
            }
            return $this->checkAndHandleFoldToOne($match);
        });
    }

    public function executeCall($match, $player)
    {
        $bet = new Bet();
        $bet->amount = 100;
        $bet->save();

        $player->chips -= 100;
        $player->save();

        $match->pot += 100;
        $match->save();

        $match->touch();
        return $bet;
    }

    public function executeRaise($match, $player)
    {
        $bet = new Bet();
        $bet->amount = 200;
        $bet->save();

        $player->chips -= 200;
        $player->save();

        $match->pot += 200;
        $match->save();

        $match->touch();
        return $bet;
    }

    public function executeAllIn($match, $player)
    {
        $bet = new Bet();
        $bet->amount = $player->chips;
        $bet->save();

        $player->chips = 0;
        $player->save();

        $match->pot += $bet->amount;
        $match->save();
        return $bet;
    }

    public function checkAndHandleFoldToOne($match)
    {
        $match->status = 'completed';
        $match->save();

        $player = Player::find(1);
        $player->wins += 1;
        $player->save();

        $match->touch();
        return true;
    }
}
`
	if err := os.WriteFile(filepath.Join(actionsDir, "BettingActions.php"), []byte(bettingActionsCode), 0644); err != nil {
		t.Fatalf("failed to write BettingActions.php: %v", err)
	}

	// 2. RoundSupport.php: Collaborator support class (synchronizeSeats performs 2 writes)
	roundSupportCode := `<?php
namespace App\Actions\QuizPoker;

use App\Models\Seat;

class RoundSupport
{
    public function synchronizeSeats($match)
    {
        $seat1 = Seat::find(1);
        $seat1->active = true;
        $seat1->save();

        $seat2 = Seat::find(2);
        $seat2->active = false;
        $seat2->save();
    }
}
`
	if err := os.WriteFile(filepath.Join(actionsDir, "RoundSupport.php"), []byte(roundSupportCode), 0644); err != nil {
		t.Fatalf("failed to write RoundSupport.php: %v", err)
	}

	// 3. TwitchAuthController.php: Redirect with static trusted base URL + query params
	twitchAuthCode := `<?php
namespace App\Http\Controllers;

use Illuminate\Http\Request;

class TwitchAuthController extends Controller
{
    public function redirect()
    {
        $query = http_build_query([
            'client_id' => 'abc',
            'redirect_uri' => 'https://example.com/callback',
            'response_type' => 'code',
            'scope' => 'user:read:email',
        ]);

        return redirect()->away('https://id.twitch.tv/oauth2/authorize?' . $query);
    }
}
`
	if err := os.WriteFile(filepath.Join(controllersDir, "TwitchAuthController.php"), []byte(twitchAuthCode), 0644); err != nil {
		t.Fatalf("failed to write TwitchAuthController.php: %v", err)
	}

	// 4. PlayerManagementController.php: TRUE POSITIVE multi-write without transaction
	playerMgmtCode := `<?php
namespace App\Http\Controllers;

use App\Models\Match;
use App\Models\Participant;
use Illuminate\Http\Request;

class PlayerManagementController extends Controller
{
    public function resetGame(Request $request)
    {
        $match = Match::create([
            'status' => 'pending',
            'code' => 'TEST',
        ]);

        Participant::create([
            'match_id' => $match->id,
            'user_id' => $request->user()->id,
        ]);

        return response()->json(['success' => true]);
    }
}
`
	if err := os.WriteFile(filepath.Join(controllersDir, "PlayerManagementController.php"), []byte(playerMgmtCode), 0644); err != nil {
		t.Fatalf("failed to write PlayerManagementController.php: %v", err)
	}

	// 5. routes/web.php: GET login (form view, no throttle needed) vs POST login (throttled)
	routesCode := `<?php
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\TwitchAuthController;
use App\Http\Controllers\PlayerManagementController;

Route::get('/quizpoker/login', [TwitchAuthController::class, 'showLoginForm']);
Route::post('/quizpoker/login', [TwitchAuthController::class, 'login'])->middleware('throttle:6,1');
`
	if err := os.WriteFile(filepath.Join(routesDir, "web.php"), []byte(routesCode), 0644); err != nil {
		t.Fatalf("failed to write web.php: %v", err)
	}

	// Run Built-in Scanner
	pCtx := ProjectContext{ProjectPath: tempDir}
	findings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("scanBuiltinsRaw failed: %v", err)
	}

	var falsePositives []string
	var foundResetGameTruePositive bool

	for _, f := range findings {
		rel := filepath.ToSlash(f.File)

		// 1. BettingActions should have 0 MultiWriteTransactionRequired
		if strings.Contains(rel, "BettingActions.php") && f.Rule == "MultiWriteTransactionRequired" {
			falsePositives = append(falsePositives, "BettingActions.php: "+f.Message)
		}
		// 2. RoundSupport should have 0 MultiWriteTransactionRequired
		if strings.Contains(rel, "RoundSupport.php") && f.Rule == "MultiWriteTransactionRequired" {
			falsePositives = append(falsePositives, "RoundSupport.php: "+f.Message)
		}
		// 3. TwitchAuthController should have 0 UnsafeExternalRedirect
		if strings.Contains(rel, "TwitchAuthController.php") && f.Rule == "UnsafeExternalRedirect" {
			falsePositives = append(falsePositives, "TwitchAuthController.php: "+f.Message)
		}
		// 4. GET /quizpoker/login should have 0 AuthRouteRateLimiting
		if strings.Contains(rel, "web.php") && f.Rule == "AuthRouteRateLimiting" {
			falsePositives = append(falsePositives, "web.php: "+f.Message)
		}
		// 5. PlayerManagementController::resetGame SHOULD be flagged (true positive)
		if strings.Contains(rel, "PlayerManagementController.php") && f.Rule == "MultiWriteTransactionRequired" {
			foundResetGameTruePositive = true
		}
	}

	if len(falsePositives) > 0 {
		t.Errorf("Unexpected false positives detected (%d):\n%s", len(falsePositives), strings.Join(falsePositives, "\n"))
	}

	if !foundResetGameTruePositive {
		t.Errorf("Expected TRUE POSITIVE MultiWriteTransactionRequired on PlayerManagementController::resetGame(), but none was reported.")
	}
}

func TestTransactionalMiddleware_SuppressesControllerMultiWrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "middleware-tx-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	controllersDir := filepath.Join(tempDir, "app", "Http", "Controllers")
	_ = os.MkdirAll(controllersDir, 0755)

	middlewareDir := filepath.Join(tempDir, "app", "Http", "Middleware")
	_ = os.MkdirAll(middlewareDir, 0755)

	routesDir := filepath.Join(tempDir, "routes")
	_ = os.MkdirAll(routesDir, 0755)

	// 1. Transactional Middleware: RecordAdminActivity wraps $next in DB::transaction
	middlewareCode := `<?php
namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Symfony\Component\HttpFoundation\Response;

class RecordAdminActivity
{
    public function handle(Request $request, Closure $next): Response
    {
        return DB::transaction(function () use ($request, $next) {
            $response = $next($request);
            return $response;
        });
    }
}
`
	if err := os.WriteFile(filepath.Join(middlewareDir, "RecordAdminActivity.php"), []byte(middlewareCode), 0644); err != nil {
		t.Fatalf("failed to write RecordAdminActivity.php: %v", err)
	}

	// 2. Controller wrapped by RecordAdminActivity in routes: PlayerManagementController
	playerMgmtCode := `<?php
namespace App\Http\Controllers;

use App\Models\Game;
use App\Models\GamePlayer;
use Illuminate\Http\Request;

class PlayerManagementController extends Controller
{
    public function resetGame(Request $request)
    {
        $game = Game::create(['status' => 'lobby']);
        GamePlayer::create(['game_id' => $game->id, 'user_id' => 1]);
        return response()->json(['success' => true]);
    }
}
`
	if err := os.WriteFile(filepath.Join(controllersDir, "PlayerManagementController.php"), []byte(playerMgmtCode), 0644); err != nil {
		t.Fatalf("failed to write PlayerManagementController.php: %v", err)
	}

	// 3. Unprotected Controller: CheckoutController without transactional middleware
	unprotectedCode := `<?php
namespace App\Http\Controllers;

use App\Models\Order;
use App\Models\Payment;
use Illuminate\Http\Request;

class CheckoutController extends Controller
{
    public function checkout(Request $request)
    {
        $order = Order::create(['total' => 100]);
        Payment::create(['order_id' => $order->id, 'amount' => 100]);
        return response()->json(['success' => true]);
    }
}
`
	if err := os.WriteFile(filepath.Join(controllersDir, "CheckoutController.php"), []byte(unprotectedCode), 0644); err != nil {
		t.Fatalf("failed to write CheckoutController.php: %v", err)
	}

	// 4. Routes: web.php attaches RecordAdminActivity to PlayerManagementController, but NOT to CheckoutController
	routesCode := `<?php
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\PlayerManagementController;
use App\Http\Controllers\CheckoutController;
use App\Http\Middleware\RecordAdminActivity;

Route::prefix('api/admin')->middleware(RecordAdminActivity::class)->group(function () {
    Route::post('game/reset', [PlayerManagementController::class, 'resetGame']);
});

Route::post('checkout', [CheckoutController::class, 'checkout']);
`
	if err := os.WriteFile(filepath.Join(routesDir, "web.php"), []byte(routesCode), 0644); err != nil {
		t.Fatalf("failed to write web.php: %v", err)
	}

	// Run scanner
	pCtx := ProjectContext{ProjectPath: tempDir}
	findings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("scanBuiltinsRaw failed: %v", err)
	}

	var foundPlayerResetFalsePositive bool
	var foundCheckoutTruePositive bool

	for _, f := range findings {
		rel := filepath.ToSlash(f.File)
		if strings.Contains(rel, "PlayerManagementController.php") && f.Rule == "MultiWriteTransactionRequired" {
			foundPlayerResetFalsePositive = true
		}
		if strings.Contains(rel, "CheckoutController.php") && f.Rule == "MultiWriteTransactionRequired" {
			foundCheckoutTruePositive = true
		}
	}

	if foundPlayerResetFalsePositive {
		t.Errorf("Expected PlayerManagementController::resetGame() to be SUPPRESSED because route is wrapped by RecordAdminActivity transactional middleware, but it was flagged.")
	}

	if !foundCheckoutTruePositive {
		t.Errorf("Expected CheckoutController::checkout() to be FLAGGED as a TRUE POSITIVE because it has no transactional middleware, but it was not reported.")
	}
}

func TestClinicalEvaluation_BranchWritesAndRedirectPolish(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hwb-test-clinical-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	controllersDir := filepath.Join(tempDir, "app", "Http", "Controllers")
	servicesDir := filepath.Join(tempDir, "app", "Services")
	jobsDir := filepath.Join(tempDir, "app", "Jobs")
	_ = os.MkdirAll(controllersDir, 0755)
	_ = os.MkdirAll(servicesDir, 0755)
	_ = os.MkdirAll(jobsDir, 0755)

	// 1. Icd10Controller: Toggle Favorite with mutually exclusive if / else branches
	icd10Code := `<?php
namespace App\Http\Controllers;
use Illuminate\Http\Request;
use App\Models\Favorite;

class Icd10Controller extends Controller
{
    public function toggleFavorite(Request $request, int $codeId)
    {
        $existing = Favorite::where('user_id', auth()->id())->where('code_id', $codeId)->first();
        if ($existing) {
            $existing->delete();
        } else {
            Favorite::create(['user_id' => auth()->id(), 'code_id' => $codeId]);
        }
        return back();
    }
}
`
	if err := os.WriteFile(filepath.Join(controllersDir, "Icd10Controller.php"), []byte(icd10Code), 0644); err != nil {
		t.Fatalf("failed to write Icd10Controller: %v", err)
	}

	// 2. TwoFactorEmailService: Early return guard branches
	twoFactorCode := `<?php
namespace App\Services;
use App\Models\User;

class TwoFactorEmailService
{
    public function updateType(User $user, string $type)
    {
        if ($type === 'email') {
            $user->update(['two_factor_type' => 'email']);
            return;
        }
        $user->update(['two_factor_type' => 'sms']);
        return;
    }
}
`
	if err := os.WriteFile(filepath.Join(servicesDir, "TwoFactorEmailService.php"), []byte(twoFactorCode), 0644); err != nil {
		t.Fatalf("failed to write TwoFactorEmailService: %v", err)
	}

	// 3. TeamInvitationService: TRUE POSITIVE multi-write (User::create + roles()->attach + invitation->update)
	invitationCode := `<?php
namespace App\Services;
use App\Models\User;
use App\Models\TeamInvitation;

class TeamInvitationService
{
    public function acceptInvitation(string $token)
    {
        $invitation = TeamInvitation::where('token', $token)->firstOrFail();
        $user = User::create(['email' => $invitation->email]);
        $user->roles()->attach($invitation->role_id);
        $invitation->update(['accepted_at' => now()]);
        return $user;
    }
}
`
	if err := os.WriteFile(filepath.Join(servicesDir, "TeamInvitationService.php"), []byte(invitationCode), 0644); err != nil {
		t.Fatalf("failed to write TeamInvitationService: %v", err)
	}

	// 4. FhirAuthorizationController: External redirect with upstream validateRedirectUri()
	fhirCode := `<?php
namespace App\Http\Controllers;
use Illuminate\Http\Request;

class FhirAuthorizationController extends Controller
{
    public function authorize(Request $request)
    {
        $redirectUri = $this->validateRedirectUri($request->input('redirect_uri'));
        return redirect()->away($redirectUri);
    }

    protected function validateRedirectUri(string $uri): string
    {
        return $uri;
    }
}
`
	if err := os.WriteFile(filepath.Join(controllersDir, "FhirAuthorizationController.php"), []byte(fhirCode), 0644); err != nil {
		t.Fatalf("failed to write FhirAuthorizationController: %v", err)
	}

	// 5. UnsafeRedirectController: Raw unvalidated external redirect
	unsafeRedirectCode := `<?php
namespace App\Http\Controllers;
use Illuminate\Http\Request;

class UnsafeRedirectController extends Controller
{
    public function redirectOut(Request $request)
    {
        return redirect()->away($request->input('target'));
    }
}
`
	if err := os.WriteFile(filepath.Join(controllersDir, "UnsafeRedirectController.php"), []byte(unsafeRedirectCode), 0644); err != nil {
		t.Fatalf("failed to write UnsafeRedirectController: %v", err)
	}

	// 6. RetryWebhookJob: Declares only public int $tries = 3;
	retryJobCode := `<?php
namespace App\Jobs;
use Illuminate\Contracts\Queue\ShouldQueue;

class RetryWebhookJob implements ShouldQueue
{
    public int $tries = 3;

    public function handle(): void
    {
    }
}
`
	if err := os.WriteFile(filepath.Join(jobsDir, "RetryWebhookJob.php"), []byte(retryJobCode), 0644); err != nil {
		t.Fatalf("failed to write RetryWebhookJob: %v", err)
	}

	// 7. StalledWebhookJob: Declares neither $tries nor $timeout
	stalledJobCode := `<?php
namespace App\Jobs;
use Illuminate\Contracts\Queue\ShouldQueue;

class StalledWebhookJob implements ShouldQueue
{
    public function handle(): void
    {
    }
}
`
	if err := os.WriteFile(filepath.Join(jobsDir, "StalledWebhookJob.php"), []byte(stalledJobCode), 0644); err != nil {
		t.Fatalf("failed to write StalledWebhookJob: %v", err)
	}

	pCtx := ProjectContext{ProjectPath: tempDir}
	findings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("scanBuiltinsRaw failed: %v", err)
	}

	var (
		foundIcd10FalsePositive         bool
		foundTwoFactorFalsePositive     bool
		foundTeamInvitationTruePositive bool
		foundFhirRedirectFalsePositive  bool
		foundUnsafeRedirectTruePositive bool
		foundRetryJobFalsePositive      bool
		foundStalledJobTruePositive     bool
	)

	for _, f := range findings {
		rel := filepath.ToSlash(f.File)

		if strings.Contains(rel, "Icd10Controller.php") && f.Rule == "MultiWriteTransactionRequired" {
			foundIcd10FalsePositive = true
		}
		if strings.Contains(rel, "TwoFactorEmailService.php") && f.Rule == "MultiWriteTransactionRequired" {
			foundTwoFactorFalsePositive = true
		}
		if strings.Contains(rel, "TeamInvitationService.php") && f.Rule == "MultiWriteTransactionRequired" {
			foundTeamInvitationTruePositive = true
		}
		if strings.Contains(rel, "FhirAuthorizationController.php") && f.Rule == "UnsafeExternalRedirect" {
			foundFhirRedirectFalsePositive = true
		}
		if strings.Contains(rel, "UnsafeRedirectController.php") && f.Rule == "UnsafeExternalRedirect" {
			foundUnsafeRedirectTruePositive = true
		}
		if strings.Contains(rel, "RetryWebhookJob.php") && f.Rule == "JobRetriesTimeout" {
			foundRetryJobFalsePositive = true
		}
		if strings.Contains(rel, "StalledWebhookJob.php") && f.Rule == "JobRetriesTimeout" {
			foundStalledJobTruePositive = true
		}
	}

	if foundIcd10FalsePositive {
		t.Errorf("Branch-aware failure: Icd10Controller::toggleFavorite() with if/else writes was erroneously flagged by MultiWriteTransactionRequired")
	}
	if foundTwoFactorFalsePositive {
		t.Errorf("Branch-aware failure: TwoFactorEmailService::updateType() with early return was erroneously flagged by MultiWriteTransactionRequired")
	}
	if !foundTeamInvitationTruePositive {
		t.Errorf("True positive failure: TeamInvitationService::acceptInvitation() was NOT flagged by MultiWriteTransactionRequired")
	}
	if foundFhirRedirectFalsePositive {
		t.Errorf("Redirect safety failure: FhirAuthorizationController::authorize() with upstream validateRedirectUri was erroneously flagged by UnsafeExternalRedirect")
	}
	if !foundUnsafeRedirectTruePositive {
		t.Errorf("Redirect safety failure: UnsafeRedirectController::redirectOut() with unvalidated raw input was NOT flagged by UnsafeExternalRedirect")
	}
	if foundRetryJobFalsePositive {
		t.Errorf("Queue job policy failure: RetryWebhookJob declaring $tries = 3 was erroneously flagged by JobRetriesTimeout")
	}
	if !foundStalledJobTruePositive {
		t.Errorf("Queue job policy failure: StalledWebhookJob declaring neither $tries nor $timeout was NOT flagged by JobRetriesTimeout")
	}
}

func TestPolicyCategorization_OptionalRules(t *testing.T) {
	rawFindings := []CodeBestPracticeFinding{
		{
			Tool:     "Workbench heuristic",
			Rule:     "QueueJobWithoutRelations",
			File:     "app/Jobs/ProcessOrder.php",
			Line:     15,
			Severity: "info",
		},
		{
			Tool:     "Workbench heuristic",
			Rule:     "StableListKeys",
			File:     "resources/js/Pages/ItemList.tsx",
			Line:     22,
			Severity: "info",
		},
		{
			Tool:     "Workbench heuristic",
			Rule:     "EmptyGuardedRisk",
			File:     "app/Models/Notice.php",
			Line:     10,
			Severity: "info",
		},
		{
			Tool:     "Workbench heuristic",
			Rule:     "MultiWriteTransactionRequired",
			File:     "app/Services/TeamInvitationService.php",
			Line:     25,
			Severity: "warning",
		},
	}

	// 1. Default options: conventions and performance advice disabled
	defaultOpts := AuditOptions{
		IncludeConventions:       false,
		IncludePerformanceAdvice: false,
	}
	filteredDefault := builtinPolicy(rawFindings, defaultOpts)
	var hasQueue, hasKeys, hasGuarded, hasTx bool
	for _, f := range filteredDefault {
		if f.Rule == "QueueJobWithoutRelations" {
			hasQueue = true
		}
		if f.Rule == "StableListKeys" {
			hasKeys = true
		}
		if f.Rule == "EmptyGuardedRisk" {
			hasGuarded = true
		}
		if f.Rule == "MultiWriteTransactionRequired" {
			hasTx = true
		}
	}

	if hasQueue {
		t.Errorf("QueueJobWithoutRelations should NOT be in default audit (performance advice disabled)")
	}
	if hasKeys {
		t.Errorf("StableListKeys should NOT be in default audit (conventions disabled)")
	}
	if hasGuarded {
		t.Errorf("EmptyGuardedRisk should NOT be in default audit (conventions disabled)")
	}
	if !hasTx {
		t.Errorf("MultiWriteTransactionRequired MUST be present in default audit")
	}

	// 2. Enable performance advice: QueueJobWithoutRelations should appear with kind 'performance' and severity 'info'
	perfOpts := AuditOptions{
		IncludeConventions:       false,
		IncludePerformanceAdvice: true,
	}
	filteredPerf := builtinPolicy(rawFindings, perfOpts)
	var foundQueuePerf bool
	for _, f := range filteredPerf {
		if f.Rule == "QueueJobWithoutRelations" {
			foundQueuePerf = true
			if f.Kind != "performance" {
				t.Errorf("QueueJobWithoutRelations kind should be 'performance', got %q", f.Kind)
			}
			if f.Severity != "info" {
				t.Errorf("QueueJobWithoutRelations severity should be 'info', got %q", f.Severity)
			}
		}
	}
	if !foundQueuePerf {
		t.Errorf("QueueJobWithoutRelations should be present when IncludePerformanceAdvice is true")
	}

	// 3. Enable conventions: StableListKeys and EmptyGuardedRisk should appear with kind 'convention' and severity 'info'
	convOpts := AuditOptions{
		IncludeConventions:       true,
		IncludePerformanceAdvice: false,
	}
	filteredConv := builtinPolicy(rawFindings, convOpts)
	var foundKeysConv, foundGuardedConv bool
	for _, f := range filteredConv {
		if f.Rule == "StableListKeys" {
			foundKeysConv = true
			if f.Kind != "convention" {
				t.Errorf("StableListKeys kind should be 'convention', got %q", f.Kind)
			}
			if f.Severity != "info" {
				t.Errorf("StableListKeys severity should be 'info', got %q", f.Severity)
			}
		}
		if f.Rule == "EmptyGuardedRisk" {
			foundGuardedConv = true
			if f.Kind != "convention" {
				t.Errorf("EmptyGuardedRisk kind should be 'convention', got %q", f.Kind)
			}
			if f.Severity != "info" {
				t.Errorf("EmptyGuardedRisk severity should be 'info', got %q", f.Severity)
			}
		}
	}
	if !foundKeysConv {
		t.Errorf("StableListKeys should be present when IncludeConventions is true")
	}
	if !foundGuardedConv {
		t.Errorf("EmptyGuardedRisk should be present when IncludeConventions is true")
	}
}

func TestInlineSuppressionComments_AndMassAssignmentRequestAll(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hwb-test-suppress-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	controllersDir := filepath.Join(tempDir, "app", "Http", "Controllers")
	_ = os.MkdirAll(controllersDir, 0755)

	// 1. Controller with @laradeck-ignore-next-line comment suppressing MultiWriteTransactionRequired
	suppressedCode := `<?php
namespace App\Http\Controllers;
use App\Models\User;
use App\Models\Profile;

class SyncController extends Controller
{
    // @laradeck-ignore-next-line: MultiWriteTransactionRequired (verified atomic via database trigger)
    public function syncEntities()
    {
        User::create(['name' => 'John']);
        Profile::create(['bio' => 'Dev']);
    }
}
`
	if err := os.WriteFile(filepath.Join(controllersDir, "SyncController.php"), []byte(suppressedCode), 0644); err != nil {
		t.Fatalf("failed to write SyncController: %v", err)
	}

	// 2. Controller with UnsafeMassAssignmentRequestAll vs safe $request->validated()
	vulnControllerCode := `<?php
namespace App\Http\Controllers;
use Illuminate\Http\Request;
use App\Models\User;

class UserController extends Controller
{
    public function store(Request $request)
    {
        // Vulnerable: passing raw $request->all() directly into create()
        User::create($request->all());
    }

    public function safeStore(Request $request)
    {
        // Safe: passing validated data
        User::create($request->validated());
    }
}
`
	if err := os.WriteFile(filepath.Join(controllersDir, "UserController.php"), []byte(vulnControllerCode), 0644); err != nil {
		t.Fatalf("failed to write UserController: %v", err)
	}

	pCtx := ProjectContext{ProjectPath: tempDir}
	findings, err := scanBuiltinsRaw(context.Background(), "php", pCtx)
	if err != nil {
		t.Fatalf("scanBuiltinsRaw failed: %v", err)
	}

	var foundSyncMultiWrite bool
	var foundUnsafeMassAssignment bool
	var foundSafeMassAssignment bool

	for _, f := range findings {
		rel := filepath.ToSlash(f.File)
		if strings.Contains(rel, "SyncController.php") && f.Rule == "MultiWriteTransactionRequired" {
			foundSyncMultiWrite = true
		}
		if strings.Contains(rel, "UserController.php") && f.Rule == "UnsafeMassAssignmentRequestAll" {
			if f.Line == 11 {
				foundUnsafeMassAssignment = true
			}
			if f.Line == 17 {
				foundSafeMassAssignment = true
			}
		}
	}

	if foundSyncMultiWrite {
		t.Errorf("Expected SyncController::syncEntities() to be SUPPRESSED via @laradeck-ignore-next-line comment, but it was flagged.")
	}
	if !foundUnsafeMassAssignment {
		t.Errorf("Expected User::create($request->all()) on line 11 to be FLAGGED as UnsafeMassAssignmentRequestAll, but it was missed.")
	}
	if foundSafeMassAssignment {
		t.Errorf("User::create($request->validated()) on line 17 was erroneously flagged as UnsafeMassAssignmentRequestAll.")
	}
}

func TestTransactionCommentsRequireExplicitSuppression(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "app", "Http", "Controllers")
	os.MkdirAll(dir, 0700)
	cases := []struct {
		comment string
		want    int
	}{
		{"", 1}, {"// TODO: add transaction here", 1}, {"// Use caller-provided values and middleware metadata", 1},
		{"// @laradeck-ignore-next-line: DifferentRule", 1}, {"// @laradeck-ignore-next-line: MultiWriteTransactionRequired", 0},
	}
	for _, tc := range cases {
		t.Run(tc.comment, func(t *testing.T) {
			code := "<?php\nclass ReviewController {\n" + tc.comment + "\npublic function store() {\nUser::create(['name'=>'a']);\nProfile::create(['name'=>'b']);\n}\n}\n"
			path := filepath.Join(dir, "ReviewController.php")
			os.WriteFile(path, []byte(code), 0600)
			findings, err := runPHPBatchAnalyzer(context.Background(), "php", []string{path}, ProjectContext{ProjectPath: root})
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for _, f := range findings {
				if f.Rule == "MultiWriteTransactionRequired" {
					n++
				}
			}
			if n != tc.want {
				t.Fatalf("comment=%q warnings=%d want=%d", tc.comment, n, tc.want)
			}
		})
	}
}
