package stripe

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/client"
)

func resourceStripeProductFeature() *schema.Resource {
	return &schema.Resource{
		ReadContext:   resourceStripeProductFeatureRead,
		CreateContext: resourceStripeProductFeatureCreate,
		DeleteContext: resourceStripeProductFeatureDelete,
		Importer: &schema.ResourceImporter{
			// Read fetches /v1/products/{product}/features/{id}, so the parent
			// product cannot be recovered from the attachment id alone. A
			// passthrough importer leaves `product` unset and the read 404s, so
			// import the composite "<product_id>/<product_feature_id>".
			StateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
				product, featureID, found := strings.Cut(d.Id(), "/")
				if !found || product == "" || featureID == "" {
					return nil, fmt.Errorf(
						"invalid import id %q: expected \"<product_id>/<product_feature_id>\"", d.Id())
				}
				if err := d.Set("product", product); err != nil {
					return nil, err
				}
				d.SetId(featureID)
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Unique identifier for the object.",
			},
			"entitlements_feature": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the Entitlements Feature the product will be attached to",
			},
			"product": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the product that this Entitlements Feature will be attached to.",
			},
			"object": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "String representing the object’s type. Objects of the same type share the same value.",
			},
			"livemode": {
				Type:     schema.TypeBool,
				Computed: true,
				Description: "Has the value true if the object exists in live mode or the value false " +
					"if the object exists in test mode",
			},
		},
	}
}

func resourceStripeProductFeatureRead(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*client.API)
	var productFeature *stripe.ProductFeature
	var err error

	productID := stripe.String(ExtractString(d, "product"))

	err = retryWithBackOff(func() error {
		productFeature, err = c.ProductFeatures.Get(d.Id(), &stripe.ProductFeatureParams{Product: productID})
		return err
	})
	switch {
	case isNotFoundErr(err):
		d.SetId("") // remove when resource does not exist
		return nil
	case err != nil:
		return diag.FromErr(err)
	}

	return CallSet(
		d.Set("entitlements_feature", productFeature.EntitlementFeature.ID),
		d.Set("product", productID),
		d.Set("object", productFeature.Object),
		d.Set("livemode", productFeature.Livemode),
	)
}

func resourceStripeProductFeatureCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*client.API)
	var productFeature *stripe.ProductFeature
	var err error

	params := &stripe.ProductFeatureParams{
		EntitlementFeature: stripe.String(ExtractString(d, "entitlements_feature")),
		Product:            stripe.String(ExtractString(d, "product")),
	}

	err = retryWithBackOff(func() error {
		productFeature, err = c.ProductFeatures.New(params)
		return err
	})
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(productFeature.ID)
	return resourceStripeProductFeatureRead(ctx, d, m)
}

func resourceStripeProductFeatureDelete(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*client.API)
	var err error

	productID := stripe.String(ExtractString(d, "product"))

	err = retryWithBackOff(func() error {
		_, err = c.ProductFeatures.Del(d.Id(), &stripe.ProductFeatureParams{Product: productID})
		return err
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
